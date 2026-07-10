package mcu_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/emergingrobotics/gopicar/internal/fake"
	"github.com/emergingrobotics/gopicar/pkg/mcu"
)

type fakeClock struct{ slept []time.Duration }

func (c *fakeClock) Sleep(_ context.Context, d time.Duration) error {
	c.slept = append(c.slept, d)
	return nil
}

func newMCU(t *testing.T, activeAddr uint8) (*mcu.MCU, *fake.Bus, *fake.Chip, *fakeClock) {
	t.Helper()
	b := fake.NewBus()
	b.OnlyAddr = &activeAddr
	b.Responses[fake.Key(activeAddr, mcu.RegFirmwareVer)] = []byte{1, 2, 3}
	chip := fake.NewChip()
	clk := &fakeClock{}
	m, err := mcu.Open(context.Background(), b, chip, mcu.Options{DeviceTreeRoot: t.TempDir(), Clock: clk})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return m, b, chip, clk
}

func TestOpenProbesActiveAddress(t *testing.T) {
	m, _, _, _ := newMCU(t, 0x16)
	if m.Addr() != 0x16 {
		t.Fatalf("Addr = %#x, want 0x16", m.Addr())
	}
	if m.HAT().Version != mcu.HATv4 {
		t.Fatalf("HAT = %v, want V4 (empty device-tree)", m.HAT().Version)
	}
}

func TestFirmwareVersion(t *testing.T) {
	m, _, _, _ := newMCU(t, 0x14)
	maj, min, pat, err := m.FirmwareVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if maj != 1 || min != 2 || pat != 3 {
		t.Fatalf("version = %d.%d.%d, want 1.2.3", maj, min, pat)
	}
}

func TestResetPulseSequence(t *testing.T) {
	m, _, chip, clk := newMCU(t, 0x14)
	if err := m.Reset(context.Background()); err != nil {
		t.Fatal(err)
	}
	pin := chip.Pins["MCURST"]
	if pin == nil {
		t.Fatal("MCURST pin was not requested")
	}
	// Reset drives LOW then HIGH (initial HIGH from request is not recorded).
	if !reflect.DeepEqual(pin.Writes, []bool{false, true}) {
		t.Fatalf("MCURST writes = %v, want [false true]", pin.Writes)
	}
	if len(clk.slept) != 2 || clk.slept[0] != 10*time.Millisecond || clk.slept[1] != 10*time.Millisecond {
		t.Fatalf("sleeps = %v, want two 10ms", clk.slept)
	}
}

type errClock struct {
	err error
	n   int
}

func (c *errClock) Sleep(_ context.Context, _ time.Duration) error {
	c.n++
	return c.err
}

func TestResetRestoresHighOnCancel(t *testing.T) {
	b := fake.NewBus()
	a := uint8(0x14)
	b.OnlyAddr = &a
	b.Responses[fake.Key(a, mcu.RegFirmwareVer)] = []byte{1, 2, 3}
	chip := fake.NewChip()
	m, err := mcu.Open(context.Background(), b, chip, mcu.Options{DeviceTreeRoot: t.TempDir(), Clock: &errClock{err: context.Canceled}})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Reset(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("Reset err = %v, want context.Canceled", err)
	}
	w := chip.Pins["MCURST"].Writes
	if len(w) == 0 || w[len(w)-1] != true {
		t.Fatalf("MCURST writes = %v; want last write true (restored HIGH)", w)
	}
}
