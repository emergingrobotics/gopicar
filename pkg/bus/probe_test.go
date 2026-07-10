package bus_test

import (
	"context"
	"testing"

	"github.com/emergingrobotics/gopicar/internal/fake"
	"github.com/emergingrobotics/gopicar/pkg/bus"
)

func TestProbeFindsResponder(t *testing.T) {
	b := fake.NewBus()
	a := uint8(0x15)
	b.OnlyAddr = &a
	got, err := bus.Probe(context.Background(), b, []uint8{0x14, 0x15, 0x16}, 0x05)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0x15 {
		t.Fatalf("addr = %#x, want 0x15", got)
	}
}

func TestProbeNoneRespond(t *testing.T) {
	b := fake.NewBus()
	a := uint8(0x99) // nothing at any probed address
	b.OnlyAddr = &a
	if _, err := bus.Probe(context.Background(), b, []uint8{0x14, 0x15, 0x16}, 0x05); err == nil {
		t.Fatal("expected error when no device responds")
	}
}
