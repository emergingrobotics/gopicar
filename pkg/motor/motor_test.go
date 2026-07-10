package motor_test

import (
	"context"
	"testing"

	"github.com/emergingrobotics/gopicar/internal/fake"
	"github.com/emergingrobotics/gopicar/pkg/motor"
)

// dutyPct decodes the last channel duty write (0x20+ch) back to a percentage.
func lastDuty(b *fake.Bus) float64 {
	last := b.Txns[len(b.Txns)-1].Write
	v := uint16(last[1])<<8 | uint16(last[2])
	return float64(v) / 4095.0 * 100.0
}

func TestSpeedRemap(t *testing.T) {
	for _, tc := range []struct {
		speed   float64
		raw     bool
		wantPct float64
	}{
		{0, false, 0},     // 0 → 0
		{100, false, 100}, // 100 → 100
		{50, false, 75},   // 50 → 50/2+50 = 75
		{50, true, 50},    // raw: no remap
		{-50, false, 75},  // magnitude remapped regardless of sign
	} {
		b := fake.NewBus()
		chip := fake.NewChip()
		m := motor.New(b, 0x14, chip, 13, "D4", motor.Calibration{})
		if err := m.Speed(context.Background(), tc.speed, tc.raw); err != nil {
			t.Fatal(err)
		}
		if got := lastDuty(b); got < tc.wantPct-0.5 || got > tc.wantPct+0.5 {
			t.Errorf("speed %v raw=%v → %.1f%%, want %.1f%%", tc.speed, tc.raw, got, tc.wantPct)
		}
	}
}

func TestDirectionPin(t *testing.T) {
	ctx := context.Background()
	// Forward (positive) → dir pin low; reverse (negative) → high.
	b, chip := fake.NewBus(), fake.NewChip()
	m := motor.New(b, 0x14, chip, 13, "D4", motor.Calibration{})
	_ = m.Speed(ctx, 60, false)
	if p := chip.Pins["D4"]; p == nil || p.Value {
		t.Error("forward should drive D4 low")
	}
	_ = m.Speed(ctx, -60, false)
	if p := chip.Pins["D4"]; p == nil || !p.Value {
		t.Error("reverse should drive D4 high")
	}
	// Invert flips the sense.
	mi := motor.New(b, 0x14, chip, 13, "D4", motor.Calibration{Invert: true})
	_ = mi.Speed(ctx, 60, false)
	if p := chip.Pins["D4"]; !p.Value {
		t.Error("inverted forward should drive D4 high")
	}
}

func TestScale(t *testing.T) {
	b, chip := fake.NewBus(), fake.NewChip()
	// Scale 0.5, raw so we see the scaled magnitude directly: 80*0.5 = 40%.
	m := motor.New(b, 0x14, chip, 13, "D4", motor.Calibration{Scale: 0.5})
	_ = m.Speed(context.Background(), 80, true)
	if got := lastDuty(b); got < 39.5 || got > 40.5 {
		t.Errorf("scaled duty = %.1f%%, want 40%%", got)
	}
}

func TestStopWritesTwice(t *testing.T) {
	b, chip := fake.NewBus(), fake.NewChip()
	m := motor.New(b, 0x14, chip, 13, "D4", motor.Calibration{})
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(b.Txns) != 2 {
		t.Fatalf("Stop wrote %d txns, want 2", len(b.Txns))
	}
}
