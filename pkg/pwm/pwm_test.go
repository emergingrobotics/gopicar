package pwm_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/emergingrobotics/gopicar/internal/fake"
	"github.com/emergingrobotics/gopicar/pkg/pwm"
)

func TestTimerIndex(t *testing.T) {
	cases := map[uint8]uint8{0: 0, 2: 0, 4: 1, 12: 3, 13: 3, 15: 3, 16: 4, 17: 4, 18: 5, 19: 6}
	for ch, want := range cases {
		if got := pwm.TimerIndex(ch); got != want {
			t.Errorf("TimerIndex(%d) = %d, want %d", ch, got, want)
		}
	}
}

func TestAngleDuty(t *testing.T) {
	// §7.2 worked example: angle 0 → 1500 µs → duty 307.
	cases := map[float64]uint16{-90: 102, 0: 307, 90: 512}
	for angle, want := range cases {
		if got := pwm.AngleDuty(angle); got != want {
			t.Errorf("AngleDuty(%v) = %d, want %d", angle, got, want)
		}
	}
	// Clamping beyond ±90.
	if pwm.AngleDuty(200) != pwm.AngleDuty(90) || pwm.AngleDuty(-200) != pwm.AngleDuty(-90) {
		t.Error("AngleDuty should clamp to ±90")
	}
}

func TestSetServoAngleByteTrace(t *testing.T) {
	b := fake.NewBus()
	if err := pwm.SetServoAngle(context.Background(), b, 0x14, 2, 0); err != nil {
		t.Fatal(err)
	}
	want := [][]byte{
		{0x44, 0x0F, 0xFF}, // timer0 ARR = 4095
		{0x40, 0x01, 0x5F}, // timer0 PSC = 351
		{0x22, 0x01, 0x33}, // channel 2 duty = 307
	}
	if len(b.Txns) != len(want) {
		t.Fatalf("got %d txns, want %d: %+v", len(b.Txns), len(want), b.Txns)
	}
	for i, w := range want {
		if b.Txns[i].Addr != 0x14 || !reflect.DeepEqual(b.Txns[i].Write, w) {
			t.Errorf("txn %d = addr %#x %v; want 0x14 %v", i, b.Txns[i].Addr, b.Txns[i].Write, w)
		}
	}
}

func TestSetDutyPercentBoundaries(t *testing.T) {
	for _, tc := range []struct {
		pct  float64
		duty []byte
	}{
		{0, []byte{0x00, 0x00}},
		{100, []byte{0x0F, 0xFF}}, // 4095
		{50, []byte{0x08, 0x00}},  // round(0.5*4095)=2048=0x0800
		{-10, []byte{0x00, 0x00}}, // clamp low
		{150, []byte{0x0F, 0xFF}}, // clamp high
	} {
		b := fake.NewBus()
		if err := pwm.SetDutyPercent(context.Background(), b, 0x14, 13, tc.pct); err != nil {
			t.Fatal(err)
		}
		want := append([]byte{0x20 + 13}, tc.duty...)
		if !reflect.DeepEqual(b.Txns[0].Write, want) {
			t.Errorf("pct %v → %v; want %v", tc.pct, b.Txns[0].Write, want)
		}
	}
}
