// Package servo drives one PiCar-X servo channel with calibration applied. A
// Servo maps a user-facing angle (0 = centered, with a per-servo sign) to the
// raw MCU angle via raw = Trim + Dir*angle, then programs the PWM channel. It
// is usable standalone against any pkg/bus.Bus.
package servo

import (
	"context"

	"github.com/emergingrobotics/gopicar/pkg/bus"
	"github.com/emergingrobotics/gopicar/pkg/pwm"
)

// Calibration is the per-servo mapping from a user-facing angle to the raw MCU
// angle. Trim is the raw angle (deg) that physically centers the servo; Dir
// (+1 or -1) orients the user-facing sign; user angles are clamped to
// [Min, Max] and the resulting raw angle is clamped to the hardware ±90°.
type Calibration struct {
	Trim float64 `json:"trim"`
	Dir  float64 `json:"dir"`
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
}

// normalize fills sane values for a zero-ish calibration so standalone/neutral
// use is safe: Dir defaults to +1, and an empty [Min,Max] becomes ±90.
func (c Calibration) normalize() Calibration {
	if c.Dir == 0 {
		c.Dir = 1
	}
	if c.Min == 0 && c.Max == 0 {
		c.Min, c.Max = -90, 90
	}
	return c
}

// Apply converts a user-facing angle to the raw MCU angle: clamps the user
// angle to [Min, Max], maps raw = Trim + Dir*angle, and clamps raw to ±90°.
func (c Calibration) Apply(userAngle float64) float64 {
	c = c.normalize()
	if userAngle < c.Min {
		userAngle = c.Min
	}
	if userAngle > c.Max {
		userAngle = c.Max
	}
	raw := c.Trim + c.Dir*userAngle
	if raw < -90 {
		raw = -90
	}
	if raw > 90 {
		raw = 90
	}
	return raw
}

// Servo is a single calibrated servo on a PWM channel.
type Servo struct {
	b    bus.Bus
	addr uint8
	ch   uint8
	cal  Calibration
}

// New returns a Servo on PWM channel ch of the MCU at addr, using cal.
func New(b bus.Bus, addr, ch uint8, cal Calibration) *Servo {
	return &Servo{b: b, addr: addr, ch: ch, cal: cal.normalize()}
}

// Channel returns the PWM channel this servo drives.
func (s *Servo) Channel() uint8 { return s.ch }

// Calibration returns the servo's calibration (normalized).
func (s *Servo) Calibration() Calibration { return s.cal }

// Angle moves the servo to a calibrated user-facing angle (0 = centered).
func (s *Servo) Angle(ctx context.Context, userAngle float64) error {
	return pwm.SetServoAngle(ctx, s.b, s.addr, s.ch, s.cal.Apply(userAngle))
}

// SetRaw moves the servo to a raw MCU angle (±90°), ignoring calibration.
func (s *Servo) SetRaw(ctx context.Context, rawAngle float64) error {
	return pwm.SetServoAngle(ctx, s.b, s.addr, s.ch, rawAngle)
}

// Center moves the servo to its calibrated center (user angle 0).
func (s *Servo) Center(ctx context.Context) error { return s.Angle(ctx, 0) }
