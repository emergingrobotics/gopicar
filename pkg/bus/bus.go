// Package bus wraps /dev/i2c-1 as a mutex-guarded, context-aware I²C bus.
// All multi-byte register writes pass explicit byte slices; there is no word
// helper that could reorder bytes (§5.3, §17).
package bus

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Bus is the low-level I²C transaction surface. Every method takes a context so
// the retry decorator's backoff is cancellable (VISION Goal 3).
type Bus interface {
	// WriteBlock writes reg followed by data: S addr W reg data… P.
	WriteBlock(ctx context.Context, addr, reg uint8, data []byte) error
	// ReadBlock writes reg then reads n bytes via a repeated START:
	// S addr W reg  Sr addr R n P.
	ReadBlock(ctx context.Context, addr, reg uint8, n int) ([]byte, error)
	// WriteRawByte writes a single byte: S addr W v P.
	WriteRawByte(ctx context.Context, addr, v uint8) error
	// Tx performs a combined write-then-read in one transaction:
	// S addr W w… Sr addr R readN P. It writes w (if non-empty), then reads
	// readN bytes via a repeated START. Needed for the ADC protocol (§5.2),
	// whose read is preceded by more than a single register byte
	// (the stock 3-byte [chn, 0, 0] frame).
	Tx(ctx context.Context, addr uint8, w []byte, readN int) ([]byte, error)
}

// Linux I²C ioctl ABI — not exported by x/sys/unix, so defined here.
const (
	i2cRDWR = 0x0707 // I2C_RDWR
	i2cMRD  = 0x0001 // I2C_M_RD (read flag)
)

// i2cMsg mirrors the kernel struct i2c_msg. On 64-bit, buf is 8-byte aligned,
// so the three uint16 fields are followed by 2 bytes of padding (added
// explicitly so the layout is unambiguous).
type i2cMsg struct {
	addr  uint16
	flags uint16
	len   uint16
	_     uint16
	buf   uintptr
}

// i2cRdwrIoctlData mirrors struct i2c_rdwr_ioctl_data.
type i2cRdwrIoctlData struct {
	msgs  uintptr
	nmsgs uint32
}

// rdwr performs a combined transaction: write w (if non-empty), then read readN
// bytes via a repeated START. It is a package var so tests substitute it and
// exercise the framing logic without hardware.
var rdwr = func(fd int, addr uint8, w []byte, readN int) ([]byte, error) {
	msgs := make([]i2cMsg, 0, 2)
	if len(w) > 0 {
		msgs = append(msgs, i2cMsg{addr: uint16(addr), flags: 0, len: uint16(len(w)), buf: uintptr(unsafe.Pointer(&w[0]))})
	}
	var r []byte
	if readN > 0 {
		r = make([]byte, readN)
		msgs = append(msgs, i2cMsg{addr: uint16(addr), flags: i2cMRD, len: uint16(readN), buf: uintptr(unsafe.Pointer(&r[0]))})
	}
	if len(msgs) == 0 {
		return nil, nil
	}
	data := i2cRdwrIoctlData{msgs: uintptr(unsafe.Pointer(&msgs[0])), nmsgs: uint32(len(msgs))}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(i2cRDWR), uintptr(unsafe.Pointer(&data)))
	runtime.KeepAlive(w)
	runtime.KeepAlive(msgs)
	if errno != 0 {
		return nil, errno
	}
	return r, nil
}

// I2C is a mutex-guarded, context-aware handle to an I²C bus device.
type I2C struct {
	mu sync.Mutex
	f  *os.File
	fd int
}

// Open opens an I²C bus device, e.g. "/dev/i2c-1". The returned *I2C
// satisfies Bus and adds Close.
func Open(path string) (*I2C, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("bus: open %s: %w", path, err)
	}
	return &I2C{f: f, fd: int(f.Fd())}, nil
}

func (b *I2C) Close() error { return b.f.Close() }

// xfer serializes access (making the whole stack goroutine-safe) and checks the
// context before issuing the blocking ioctl.
func (b *I2C) xfer(ctx context.Context, addr uint8, w []byte, readN int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return rdwr(b.fd, addr, w, readN)
}

func (b *I2C) WriteBlock(ctx context.Context, addr, reg uint8, data []byte) error {
	w := append([]byte{reg}, data...)
	_, err := b.xfer(ctx, addr, w, 0)
	return err
}

func (b *I2C) ReadBlock(ctx context.Context, addr, reg uint8, n int) ([]byte, error) {
	return b.xfer(ctx, addr, []byte{reg}, n)
}

func (b *I2C) WriteRawByte(ctx context.Context, addr, v uint8) error {
	_, err := b.xfer(ctx, addr, []byte{v}, 0)
	return err
}

func (b *I2C) Tx(ctx context.Context, addr uint8, w []byte, readN int) ([]byte, error) {
	return b.xfer(ctx, addr, w, readN)
}

// Compile-time check that *I2C satisfies Bus.
var _ Bus = (*I2C)(nil)
