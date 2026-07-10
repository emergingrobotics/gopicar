// Package motor drives one PiCar-X rear motor: a PWM channel for speed and a
// GPIO pin for direction. It reproduces the stock set_motor_speed behavior — a
// signed speed sets the direction pin and |speed| is remapped 1..100 → 50..100
// (the gearbox will not turn below ~50% duty) — with an optional per-motor
// calibration. Usable standalone against a pkg/bus.Bus + pkg/gpio.Chip.
package motor

import (
	"context"
	"time"

	"github.com/emergingrobotics/gopicar/pkg/bus"
	"github.com/emergingrobotics/gopicar/pkg/gpio"
	"github.com/emergingrobotics/gopicar/pkg/pwm"
)

// Calibration adjusts one motor so a pair can be matched. Scale multiplies the
// requested magnitude before the gearbox remap (default 1.0 via normalize);
// Invert flips the direction-pin sense for a motor wired backwards.
type Calibration struct {
	Scale  float64 `json:"scale"`
	Invert bool    `json:"invert"`
}

func (c Calibration) normalize() Calibration {
	if c.Scale == 0 {
		c.Scale = 1
	}
	return c
}

// Motor is one rear motor: PWM channel pwmChan and direction pin dirPin.
type Motor struct {
	b       bus.Bus
	addr    uint8
	chip    gpio.Chip
	pwmChan uint8
	dirPin  string
	cal     Calibration
}

// New returns a Motor. pwmChan is the speed PWM channel; dirPin is the HAT pin
// name controlling direction. The motor timer must be set up once via
// SetupTimer before the first Speed call.
func New(b bus.Bus, addr uint8, chip gpio.Chip, pwmChan uint8, dirPin string, cal Calibration) *Motor {
	return &Motor{b: b, addr: addr, chip: chip, pwmChan: pwmChan, dirPin: dirPin, cal: cal.normalize()}
}

// SetupTimer programs the motor PWM timer (period 4095, prescaler 10) for this
// motor's channel. Call once before driving.
func (m *Motor) SetupTimer(ctx context.Context) error {
	return pwm.SetupMotorTimer(ctx, m.b, m.addr, m.pwmChan)
}

// Speed drives the motor at a signed speed in [-100, 100]; the sign sets the
// direction. |speed| is scaled by the calibration and, unless raw is true,
// remapped 1..100 → 50..100 (§6.3). speed 0 stops (0% duty).
func (m *Motor) Speed(ctx context.Context, speed float64, raw bool) error {
	if speed < -100 {
		speed = -100
	}
	if speed > 100 {
		speed = 100
	}
	reverse := speed < 0
	mag := speed
	if mag < 0 {
		mag = -mag
	}
	mag *= m.cal.Scale
	if mag > 100 {
		mag = 100
	}
	if !raw && mag != 0 {
		mag = mag/2 + 50
	}

	dir, err := m.chip.RequestOutput(m.dirPin, false)
	if err != nil {
		return err
	}
	defer dir.Close()
	// direction < 0 → DIR high, else low (§6.3); Invert flips that sense.
	if err := dir.Write(reverse != m.cal.Invert); err != nil {
		return err
	}
	return pwm.SetDutyPercent(ctx, m.b, m.addr, m.pwmChan, mag)
}

// Stop writes 0% duty twice with a short gap, matching the defensive stock
// stop() against a single lost I²C transaction (§6.5).
func (m *Motor) Stop(ctx context.Context) error {
	for i := 0; i < 2; i++ {
		if err := pwm.SetDutyPercent(ctx, m.b, m.addr, m.pwmChan, 0); err != nil {
			return err
		}
		time.Sleep(2 * time.Millisecond)
	}
	return nil
}
