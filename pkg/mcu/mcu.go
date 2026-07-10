package mcu

import (
	"context"
	"fmt"
	"time"

	"github.com/emergingrobotics/gopicar/pkg/bus"
	"github.com/emergingrobotics/gopicar/pkg/gpio"
)

// Clock abstracts sleeping so tests run without real delays.
type Clock interface {
	Sleep(ctx context.Context, d time.Duration) error
}

type realClock struct{}

func (realClock) Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// MCU is a handle to the Robot HAT co-processor.
type MCU struct {
	bus   bus.Bus
	chip  gpio.Chip
	hat   HAT
	addr  uint8
	clock Clock
}

// Options configures Open.
type Options struct {
	DeviceTreeRoot string // default "/"
	Clock          Clock  // default realClock
}

// Open detects the HAT and probes the active I²C address. It does NOT reset the
// MCU — reset ordering relative to ADC construction is the Device's job (§13).
func Open(ctx context.Context, b bus.Bus, chip gpio.Chip, opts Options) (*MCU, error) {
	root := opts.DeviceTreeRoot
	if root == "" {
		root = "/"
	}
	clk := opts.Clock
	if clk == nil {
		clk = realClock{}
	}
	hat := DetectHAT(root)
	// Probe runs through whatever bus the caller supplies (retry decorator
	// included). A transient bus glitch during startup probing is worth
	// retrying; on a genuinely HAT-absent board each address exhausts its
	// retries before we advance — bounded startup latency we accept.
	addr, err := bus.Probe(ctx, b, ProbeAddrs, RegFirmwareVer)
	if err != nil {
		return nil, fmt.Errorf("mcu: probe: %w", err)
	}
	return &MCU{bus: b, chip: chip, hat: hat, addr: addr, clock: clk}, nil
}

func (m *MCU) HAT() HAT    { return m.hat }
func (m *MCU) Addr() uint8 { return m.addr }

// FirmwareVersion reads the 3-byte firmware version (§15).
func (m *MCU) FirmwareVersion(ctx context.Context) (major, minor, patch uint8, err error) {
	b, err := m.bus.ReadBlock(ctx, m.addr, RegFirmwareVer, 3)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("mcu: read firmware version: %w", err)
	}
	if len(b) < 3 {
		return 0, 0, 0, fmt.Errorf("mcu: short firmware read: got %d bytes, want 3", len(b))
	}
	return b[0], b[1], b[2], nil
}

// Reset pulses MCURST (GPIO5) LOW ≥10 ms then HIGH ≥10 ms (§5.4).
//
// WARNING: resetting the MCU poisons any ADC state established beforehand
// (§17) — ADC reads return the constant tuple [2571, 3085, 3599] until the ADC
// objects are rebuilt. After initialization, reach reset only through
// picarx.Device.Recover, never directly.
func (m *MCU) Reset(ctx context.Context) error {
	pin, err := m.chip.RequestOutput("MCURST", true)
	if err != nil {
		return fmt.Errorf("mcu: reset: request MCURST: %w", err)
	}
	defer pin.Close()
	if err := pin.Write(false); err != nil {
		return fmt.Errorf("mcu: reset: drive low: %w", err)
	}
	if err := m.clock.Sleep(ctx, 10*time.Millisecond); err != nil {
		_ = pin.Write(true) // best-effort: don't leave the MCU held in reset on cancel
		return err
	}
	if err := pin.Write(true); err != nil {
		return fmt.Errorf("mcu: reset: drive high: %w", err)
	}
	if err := m.clock.Sleep(ctx, 10*time.Millisecond); err != nil {
		return err
	}
	return nil
}
