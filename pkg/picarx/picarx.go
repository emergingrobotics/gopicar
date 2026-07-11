package picarx

import (
	"context"
	"fmt"
	"time"

	"github.com/emergingrobotics/gopicar/pkg/adc"
	"github.com/emergingrobotics/gopicar/pkg/bus"
	"github.com/emergingrobotics/gopicar/pkg/gpio"
	"github.com/emergingrobotics/gopicar/pkg/mcu"
	"github.com/emergingrobotics/gopicar/pkg/motor"
	"github.com/emergingrobotics/gopicar/pkg/servo"
	"github.com/emergingrobotics/gopicar/pkg/ultrasonic"
)

// Default device paths.
const (
	DefaultI2CDev   = "/dev/i2c-1"
	DefaultGPIOChip = "gpiochip0"
)

// Options configures Open. The zero value opens the real robot on the default
// devices with a neutral calibration and stock wiring.
type Options struct {
	I2CDev      string          // default DefaultI2CDev
	GPIOChip    string          // default DefaultGPIOChip
	Calibration Calibration     // default NeutralCalibration()
	Wiring      Wiring          // default DefaultWiring()
	Retry       bus.RetryPolicy // applied only when Open creates the bus; zero → DefaultRetryPolicy
	MCU         mcu.Options     // passthrough (DeviceTreeRoot, Clock)

	// Advanced/testing overrides. If Bus or Chip is non-nil, Open uses it
	// instead of opening the corresponding device (and does not close it).
	Bus  bus.Bus
	Chip gpio.Chip
}

// PiCarX is a handle to a whole PiCar-X: servos, motors, and sensors on one
// Robot HAT. It is safe for concurrent use (the underlying bus is mutex-guarded).
// Create with Open; release with Close.
type PiCarX struct {
	b        bus.Bus
	chip     gpio.Chip
	mc       *mcu.MCU
	wiring   Wiring
	adc      *adc.ADC
	us       *ultrasonic.Sensor
	servos   map[string]*servo.Servo
	motors   map[string]*motor.Motor
	closeFns []func() error
}

// Open builds the full stack (bus → retry → MCU probe → device objects) and
// returns a ready PiCarX. It does not reset the MCU.
func Open(ctx context.Context, opts Options) (*PiCarX, error) {
	if opts.I2CDev == "" {
		opts.I2CDev = DefaultI2CDev
	}
	if opts.GPIOChip == "" {
		opts.GPIOChip = DefaultGPIOChip
	}
	if (opts.Calibration == Calibration{}) {
		opts.Calibration = NeutralCalibration()
	}
	opts.Wiring = opts.Wiring.withDefaults()

	p := &PiCarX{wiring: opts.Wiring, servos: map[string]*servo.Servo{}, motors: map[string]*motor.Motor{}}

	// Bus: use the override, or open the real device and wrap with retry.
	if opts.Bus != nil {
		p.b = opts.Bus
	} else {
		raw, err := bus.Open(opts.I2CDev)
		if err != nil {
			return nil, err
		}
		policy := opts.Retry
		if policy.Attempts == 0 {
			policy = bus.DefaultRetryPolicy()
		}
		p.b = bus.WithRetry(raw, policy)
		p.closeFns = append(p.closeFns, raw.Close)
	}

	// GPIO chip: override or real.
	if opts.Chip != nil {
		p.chip = opts.Chip
	} else {
		chip, err := gpio.Open(opts.GPIOChip)
		if err != nil {
			p.closeAll()
			return nil, err
		}
		p.chip = chip
		p.closeFns = append(p.closeFns, chip.Close)
	}

	mc, err := mcu.Open(ctx, p.b, p.chip, opts.MCU)
	if err != nil {
		p.closeAll()
		return nil, err
	}
	p.mc = mc
	addr := mc.Addr()

	// Device objects.
	p.adc = adc.New(p.b, addr)
	p.us = ultrasonic.New(p.chip, opts.Wiring.TrigPin, opts.Wiring.EchoPin)
	p.servos["steer"] = servo.New(p.b, addr, opts.Wiring.SteerChan, opts.Calibration.Steer)
	p.servos["pan"] = servo.New(p.b, addr, opts.Wiring.PanChan, opts.Calibration.Pan)
	p.servos["tilt"] = servo.New(p.b, addr, opts.Wiring.TiltChan, opts.Calibration.Tilt)
	p.motors["left"] = motor.New(p.b, addr, p.chip, opts.Wiring.LeftMotorChan, opts.Wiring.LeftDirPin, opts.Calibration.LeftMotor)
	p.motors["right"] = motor.New(p.b, addr, p.chip, opts.Wiring.RightMotorChan, opts.Wiring.RightDirPin, opts.Calibration.RightMotor)

	// Program the shared motor timer once.
	if err := p.motors["left"].SetupTimer(ctx); err != nil {
		p.closeAll()
		return nil, fmt.Errorf("picarx: motor timer setup: %w", err)
	}
	return p, nil
}

func (p *PiCarX) closeAll() error {
	var first error
	// Close in reverse order of acquisition.
	for i := len(p.closeFns) - 1; i >= 0; i-- {
		if err := p.closeFns[i](); err != nil && first == nil {
			first = err
		}
	}
	p.closeFns = nil
	return first
}

// Close releases the GPIO chip and bus (those Open created). It is idempotent.
func (p *PiCarX) Close() error { return p.closeAll() }

// --- Escape hatches to the granular devices --------------------------------

// Servo returns the named servo ("steer", "pan", or "tilt"), or nil if unknown.
func (p *PiCarX) Servo(name string) *servo.Servo { return p.servos[name] }

// Motor returns the named motor ("left" or "right"), or nil if unknown.
func (p *PiCarX) Motor(name string) *motor.Motor { return p.motors[name] }

// ADC returns the analog-input reader.
func (p *PiCarX) ADC() *adc.ADC { return p.adc }

// Ultrasonic returns the distance sensor.
func (p *PiCarX) Ultrasonic() *ultrasonic.Sensor { return p.us }

// MCU returns the co-processor handle (firmware/HAT/reset).
func (p *PiCarX) MCU() *mcu.MCU { return p.mc }

// Bus returns the underlying I²C bus. Advanced escape hatch for driving raw
// PWM channels or registers not covered by the higher-level methods.
func (p *PiCarX) Bus() bus.Bus { return p.b }

// GPIO returns the underlying GPIO chip.
func (p *PiCarX) GPIO() gpio.Chip { return p.chip }

// --- Servos ----------------------------------------------------------------

// SetDir steers the front wheels to a calibrated angle (+right / -left, 0 straight).
func (p *PiCarX) SetDir(ctx context.Context, deg float64) error {
	return p.servos["steer"].Angle(ctx, deg)
}

// SetCamPan aims the camera left/right (+right / -left, 0 forward).
func (p *PiCarX) SetCamPan(ctx context.Context, deg float64) error {
	return p.servos["pan"].Angle(ctx, deg)
}

// SetCamTilt aims the camera up/down (+up / -down, 0 level).
func (p *PiCarX) SetCamTilt(ctx context.Context, deg float64) error {
	return p.servos["tilt"].Angle(ctx, deg)
}

// --- Motors / drive --------------------------------------------------------

// Forward drives both motors forward at pct (0..100).
func (p *PiCarX) Forward(ctx context.Context, pct float64) error {
	return p.bothMotors(ctx, pct)
}

// Backward drives both motors backward at pct (0..100).
func (p *PiCarX) Backward(ctx context.Context, pct float64) error {
	return p.bothMotors(ctx, -pct)
}

func (p *PiCarX) bothMotors(ctx context.Context, signed float64) error {
	if err := p.motors["left"].Speed(ctx, signed, false); err != nil {
		return err
	}
	return p.motors["right"].Speed(ctx, signed, false)
}

// Stop stops both motors.
func (p *PiCarX) Stop(ctx context.Context) error {
	if err := p.motors["left"].Stop(ctx); err != nil {
		return err
	}
	return p.motors["right"].Stop(ctx)
}

// Spin rotates the car in place by driving the rear motors in opposite
// directions: pct>0 spins right (clockwise viewed from above), pct<0 spins left.
// pct is a signed percentage (-100..100). It is independent of the steering
// angle and overrides any prior Forward/Backward, so it works even while moving.
func (p *PiCarX) Spin(ctx context.Context, pct float64) error {
	if err := p.motors["left"].Speed(ctx, pct, false); err != nil {
		return err
	}
	return p.motors["right"].Speed(ctx, -pct, false)
}

// Ramp linearly changes both motors' speed from `from` to `to` (signed pct)
// over dur, in ~20 ms steps. Honors context cancellation. A negative speed is
// reverse; ensure the path is clear before calling.
func (p *PiCarX) Ramp(ctx context.Context, from, to float64, dur time.Duration) error {
	const step = 20 * time.Millisecond
	steps := int(dur / step)
	if steps < 1 {
		steps = 1
	}
	for i := 0; i <= steps; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		v := from + (to-from)*float64(i)/float64(steps)
		if err := p.bothMotors(ctx, v); err != nil {
			return err
		}
		if i < steps {
			t := time.NewTimer(step)
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-t.C:
			}
		}
	}
	return nil
}

// --- Sensors ---------------------------------------------------------------

// Battery returns the pack voltage in volts.
func (p *PiCarX) Battery(ctx context.Context) (float64, error) { return p.adc.Battery(ctx) }

// Grayscale returns the three line-sensor channels as [L, M, R].
func (p *PiCarX) Grayscale(ctx context.Context) ([3]int, error) { return p.adc.Grayscale(ctx) }

// Distance returns the ultrasonic distance in cm (timeout bounds max range).
func (p *PiCarX) Distance(ctx context.Context, timeout time.Duration) (float64, error) {
	return p.us.Distance(ctx, timeout)
}

// LineStatus reports, per grayscale channel [L, M, R], whether it reads as
// "over the line" — i.e. at or below the corresponding reference threshold
// (a darker line reads lower than the surface). ref is caller-supplied because
// it depends on the surface and lighting.
func (p *PiCarX) LineStatus(ctx context.Context, ref [3]int) ([3]bool, error) {
	var out [3]bool
	g, err := p.adc.Grayscale(ctx)
	if err != nil {
		return out, err
	}
	for i := 0; i < 3; i++ {
		out[i] = g[i] <= ref[i]
	}
	return out, nil
}

// CliffStatus reports whether the robot is at an edge/cliff: true when ALL
// three grayscale channels read at or below their reference (the whole sensor
// bar sees the drop-off). ref is caller-supplied.
func (p *PiCarX) CliffStatus(ctx context.Context, ref [3]int) (bool, error) {
	s, err := p.LineStatus(ctx, ref)
	if err != nil {
		return false, err
	}
	return s[0] && s[1] && s[2], nil
}

// --- Identity / recovery ---------------------------------------------------

// FirmwareVersion reads the MCU firmware version.
func (p *PiCarX) FirmwareVersion(ctx context.Context) (major, minor, patch uint8, err error) {
	return p.mc.FirmwareVersion(ctx)
}

// HAT returns the detected HAT descriptor.
func (p *PiCarX) HAT() mcu.HAT { return p.mc.HAT() }

// Addr returns the MCU's active I²C address.
func (p *PiCarX) Addr() uint8 { return p.mc.Addr() }

// Reset pulses the MCU reset line.
//
// WARNING: resetting poisons live ADC state (§17) — ADC reads return a constant
// tuple until the ADC is exercised afresh. Use only for recovery.
func (p *PiCarX) Reset(ctx context.Context) error { return p.mc.Reset(ctx) }
