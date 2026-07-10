package servo_test

import (
	"context"
	"testing"

	"github.com/emergingrobotics/gopicar/internal/fake"
	"github.com/emergingrobotics/gopicar/pkg/servo"
)

func TestApplyMeasuredCenters(t *testing.T) {
	// The 2026-07-10 measured values for this robot.
	steer := servo.Calibration{Trim: -58, Dir: 1, Min: -30, Max: 30}
	pan := servo.Calibration{Trim: -11, Dir: -1, Min: -80, Max: 80}
	tilt := servo.Calibration{Trim: 25, Dir: -1, Min: -40, Max: 85}

	if got := steer.Apply(0); got != -58 {
		t.Errorf("steer center = %v, want -58", got)
	}
	if got := pan.Apply(0); got != -11 {
		t.Errorf("pan center = %v, want -11", got)
	}
	if got := tilt.Apply(0); got != 25 {
		t.Errorf("tilt center = %v, want 25", got)
	}
	// steer +20 (right) → -58 + 20 = -38.
	if got := steer.Apply(20); got != -38 {
		t.Errorf("steer +20 = %v, want -38", got)
	}
	// pan +45 (right, dir -1) → -11 - 45 = -56.
	if got := pan.Apply(45); got != -56 {
		t.Errorf("pan +45 = %v, want -56", got)
	}
}

func TestApplyClampsUserThenRaw(t *testing.T) {
	steer := servo.Calibration{Trim: -58, Dir: 1, Min: -30, Max: 30}
	// User beyond Max clamps to +30 → -28.
	if got := steer.Apply(100); got != -28 {
		t.Errorf("steer +100 clamped = %v, want -28", got)
	}
	// A calibration that would exceed hardware ±90 clamps at raw level.
	hot := servo.Calibration{Trim: 80, Dir: 1, Min: -90, Max: 90}
	if got := hot.Apply(30); got != 90 {
		t.Errorf("raw clamp = %v, want 90", got)
	}
}

func TestApplyZeroValueIsSafe(t *testing.T) {
	// Zero-value calibration normalizes to Dir=+1, ±90 range (identity mapping).
	var neutral servo.Calibration
	if got := neutral.Apply(30); got != 30 {
		t.Errorf("neutral.Apply(30) = %v, want 30", got)
	}
	if got := neutral.Apply(0); got != 0 {
		t.Errorf("neutral.Apply(0) = %v, want 0", got)
	}
}

func TestAngleWritesCalibratedRaw(t *testing.T) {
	b := fake.NewBus()
	s := servo.New(b, 0x14, 2, servo.Calibration{Trim: -58, Dir: 1, Min: -30, Max: 30})
	if err := s.Center(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Last txn is the channel duty write for raw -58° on channel 2 (reg 0x22).
	last := b.Txns[len(b.Txns)-1]
	if last.Write[0] != 0x22 {
		t.Fatalf("channel write reg = %#x, want 0x22", last.Write[0])
	}
	// raw -58 → duty: 500 + (32/180)*2000 = 855.5 µs → round(855.5/20000*4095)=175 = 0x00AF.
	if last.Write[1] != 0x00 || last.Write[2] != 0xAF {
		t.Errorf("duty bytes = %#x %#x, want 0x00 0xAF", last.Write[1], last.Write[2])
	}
}
