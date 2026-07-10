// Package pwm programs the Robot HAT MCU's PWM timers and channels: the
// servo-frame timer setup, angle→pulse-width conversion, and percentage duty
// writes. It is the shared low-level layer beneath pkg/servo and pkg/motor and
// can be used standalone against any pkg/bus.Bus.
//
// All 16-bit register values are written big-endian as an explicit [hi, lo]
// block — never a word-write helper that could reorder the bytes (§5.3, §17).
package pwm

import (
	"context"
	"math"

	"github.com/emergingrobotics/gopicar/pkg/bus"
	"github.com/emergingrobotics/gopicar/pkg/mcu"
)

// Timer/frame constants (§7.1, §5.3).
const (
	// ServoPeriodARR is the 16-bit timer top for the 50 Hz servo frame.
	ServoPeriodARR uint16 = 4095
	// ServoPSCReg is the prescaler register value for 50 Hz: computed PSC 352
	// minus 1 per the STM32 convention.
	ServoPSCReg uint16 = 351

	// MotorPeriodARR matches the stock picarx.py pin.period(4095).
	MotorPeriodARR uint16 = 4095
	// MotorPSCReg is pin.prescaler(10) written as 10-1.
	MotorPSCReg uint16 = 9

	// FrameMicros is the servo frame length in microseconds (20 ms @ 50 Hz).
	FrameMicros = 20000.0
	// MinPulseMicros / MaxPulseMicros are the pulse widths for -90°/+90° (§7.2).
	MinPulseMicros = 500.0
	MaxPulseMicros = 2500.0
)

// TimerIndex returns the timer serving a PWM channel (§5.3): channels 0..15 map
// to timers 0..3 (ch/4); 16/17→4, 18→5, 19→6.
func TimerIndex(ch uint8) uint8 {
	switch {
	case ch < 16:
		return ch / 4
	case ch == 16 || ch == 17:
		return 4
	case ch == 18:
		return 5
	default:
		return 6
	}
}

// Write16 writes a 16-bit value big-endian to reg as an explicit [hi, lo] block.
func Write16(ctx context.Context, b bus.Bus, addr, reg uint8, v uint16) error {
	return b.WriteBlock(ctx, addr, reg, []byte{byte(v >> 8), byte(v)})
}

// SetTimer programs a timer's period (ARR) then prescaler (PSC). Timers 0..3 use
// the 0x44/0x40 register base; V5 timers 4..6 use 0x54/0x50 (§5.3).
func SetTimer(ctx context.Context, b bus.Bus, addr, timer uint8, psc, arr uint16) error {
	pscBase, arrBase := mcu.RegTimerPSCBase, mcu.RegTimerARRBase
	if timer >= 4 {
		pscBase, arrBase = mcu.RegTimerPSC2Base, mcu.RegTimerARR2Base
		timer -= 4
	}
	if err := Write16(ctx, b, addr, arrBase+timer, arr); err != nil {
		return err
	}
	return Write16(ctx, b, addr, pscBase+timer, psc)
}

// AngleDuty converts a servo angle in degrees (clamped to ±90) to the 16-bit
// duty value for a 50 Hz frame (§7.2). Exposed for testing and reuse.
func AngleDuty(angle float64) uint16 {
	if angle < -90 {
		angle = -90
	}
	if angle > 90 {
		angle = 90
	}
	us := MinPulseMicros + (angle+90.0)/180.0*(MaxPulseMicros-MinPulseMicros)
	return uint16(math.Round(us / FrameMicros * float64(ServoPeriodARR)))
}

// SetServoAngle programs the channel's timer for a 50 Hz frame (idempotent,
// matching the stock Servo constructor) then writes the pulse-width duty (§7.2).
func SetServoAngle(ctx context.Context, b bus.Bus, addr, ch uint8, angle float64) error {
	if err := SetTimer(ctx, b, addr, TimerIndex(ch), ServoPSCReg, ServoPeriodARR); err != nil {
		return err
	}
	return Write16(ctx, b, addr, mcu.RegPWMChanBase+ch, AngleDuty(angle))
}

// SetupMotorTimer programs the timer serving the rear-motor channels as the
// stock driver does (period 4095, prescaler 10).
func SetupMotorTimer(ctx context.Context, b bus.Bus, addr, motorChan uint8) error {
	return SetTimer(ctx, b, addr, TimerIndex(motorChan), MotorPSCReg, MotorPeriodARR)
}

// SetDutyPercent writes a duty as a percentage (0..100) of the motor timer
// period to a channel (§5.3).
func SetDutyPercent(ctx context.Context, b bus.Bus, addr, ch uint8, pct float64) error {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	duty := uint16(math.Round(pct / 100.0 * float64(MotorPeriodARR)))
	return Write16(ctx, b, addr, mcu.RegPWMChanBase+ch, duty)
}
