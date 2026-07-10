// Package fake provides in-memory bus.Bus and gpio.Chip implementations plus a
// device-tree fixture, so the whole driver stack is testable without hardware.
package fake

import (
	"context"
	"fmt"

	"github.com/emergingrobotics/gopicar/pkg/bus"
)

// Txn records a single recorded transaction.
type Txn struct {
	Addr  uint8
	Write []byte
	Read  []byte
}

// Key builds the Responses map key for (addr, reg).
func Key(addr, reg uint8) uint16 { return uint16(addr)<<8 | uint16(reg) }

// Bus is an in-memory bus.Bus that records every transaction.
type Bus struct {
	Txns      []Txn
	Responses map[uint16][]byte // keyed by Key(addr, reg) → bytes returned by ReadBlock
	ReadQueue map[uint8][]byte  // per-addr FIFO for bare reads (Tx with empty write)
	FailFirst int               // first N calls return an error (exercise retry)
	Err       error             // error returned by FailFirst; default if nil
	OnlyAddr  *uint8            // if set, calls to other addresses error (simulate NACK)
}

func NewBus() *Bus {
	return &Bus{Responses: map[uint16][]byte{}, ReadQueue: map[uint8][]byte{}}
}

// EnqueueRead appends bytes to the per-address bare-read FIFO. A bare read
// (Tx with no write payload, as pkg/adc uses for the two result bytes) pops
// from the front of this queue.
func (b *Bus) EnqueueRead(addr uint8, data ...byte) {
	if b.ReadQueue == nil {
		b.ReadQueue = map[uint8][]byte{}
	}
	b.ReadQueue[addr] = append(b.ReadQueue[addr], data...)
}

func (b *Bus) check(ctx context.Context, addr uint8) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.OnlyAddr != nil && *b.OnlyAddr != addr {
		return fmt.Errorf("fake: no device at %#x", addr)
	}
	if b.FailFirst > 0 {
		b.FailFirst--
		if b.Err != nil {
			return b.Err
		}
		return fmt.Errorf("fake: injected error")
	}
	return nil
}

func (b *Bus) WriteBlock(ctx context.Context, addr, reg uint8, data []byte) error {
	if err := b.check(ctx, addr); err != nil {
		return err
	}
	b.Txns = append(b.Txns, Txn{Addr: addr, Write: append([]byte{reg}, data...)})
	return nil
}

func (b *Bus) ReadBlock(ctx context.Context, addr, reg uint8, n int) ([]byte, error) {
	if err := b.check(ctx, addr); err != nil {
		return nil, err
	}
	out := make([]byte, n)
	copy(out, b.Responses[Key(addr, reg)])
	b.Txns = append(b.Txns, Txn{Addr: addr, Write: []byte{reg}, Read: out})
	return out, nil
}

func (b *Bus) WriteRawByte(ctx context.Context, addr, v uint8) error {
	if err := b.check(ctx, addr); err != nil {
		return err
	}
	b.Txns = append(b.Txns, Txn{Addr: addr, Write: []byte{v}})
	return nil
}

func (b *Bus) Tx(ctx context.Context, addr uint8, w []byte, readN int) ([]byte, error) {
	if err := b.check(ctx, addr); err != nil {
		return nil, err
	}
	out := make([]byte, readN)
	if len(w) > 0 {
		// Combined write-then-read: response keyed by the register byte.
		copy(out, b.Responses[Key(addr, w[0])])
	} else if q := b.ReadQueue[addr]; len(q) > 0 {
		// Bare read: pop from the per-address FIFO.
		n := copy(out, q)
		b.ReadQueue[addr] = q[n:]
	}
	b.Txns = append(b.Txns, Txn{Addr: addr, Write: append([]byte(nil), w...), Read: out})
	return out, nil
}

var _ bus.Bus = (*Bus)(nil)
