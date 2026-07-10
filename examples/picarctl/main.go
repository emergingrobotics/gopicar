// Command picarctl is the reference example for the gopicar library: a small
// CLI that smoke-tests a PiCar-X by driving github.com/emergingrobotics/gopicar
// through its high-level pkg/picarx facade (and the granular packages where a
// command needs finer control). It also demonstrates one way to persist servo
// calibration — see calibstore.go — which the library itself deliberately does
// not do.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/emergingrobotics/gopicar/pkg/gpio"
	"github.com/emergingrobotics/gopicar/pkg/picarx"
)

const (
	i2cPath  = "/dev/i2c-1"
	gpioChip = "gpiochip0"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func printUsage(out io.Writer) {
	fmt.Fprint(out, `picarctl — PiCar-X smoke tests (reference example for gopicar)

Identity / bus:
  picarctl ping-mcu            probe the MCU and print its I²C address + firmware version
  picarctl hat-info            print detected HAT version, speaker-EN pin, motor mode, UUID
  picarctl blink [--pin D14] [--count 5]
                               blink a GPIO (default the user LED, D14)
  picarctl reset-mcu           pulse the MCU reset line
                               (WARNING: poisons live ADC state — see §17)

Actuators:
  picarctl servo (--which pan|tilt|steer | --channel P0) --angle N [--raw]
                               move a servo. With --which, --angle is a calibrated
                               user angle (0 = centered; +right/+up); --raw sends it
                               as the raw MCU angle. --channel is always raw.
  picarctl calibrate <show | set --which S --trim N [--dir ±1] [--min N] [--max N] | save-defaults>
                               view or edit the persisted servo calibration
  picarctl motor --which left|right --speed -100..100 [--raw] [--duration 1s]
                               drive one rear motor, then auto-stop
  picarctl stop                stop both motors

Sensors:
  picarctl read-adc [--channel A0]     read one raw ADC channel (A0..A4)
  picarctl read-battery                read the battery voltage (ADC A4 × 3)
  picarctl read-grayscale              read the 3 grayscale channels (A0/A1/A2)
  picarctl distance [--count 1] [--timeout 20ms]
                               measure ultrasonic distance in cm
`)
}

func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		printUsage(out)
		return nil
	}
	// Cancel the run on Ctrl-C / SIGTERM so a timed motor command can stop the
	// motor gracefully instead of leaving it spinning (see cmdMotor).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch args[0] {
	case "ping-mcu":
		return cmdPingMCU(ctx, out)
	case "hat-info":
		return cmdHATInfo(ctx, out)
	case "blink":
		return cmdBlink(ctx, args[1:], out)
	case "reset-mcu":
		return cmdResetMCU(ctx, out)
	case "servo":
		return cmdServo(ctx, args[1:], out)
	case "calibrate":
		return cmdCalibrate(ctx, args[1:], out)
	case "motor":
		return cmdMotor(ctx, args[1:], out)
	case "stop":
		return cmdStop(ctx, out)
	case "read-adc":
		return cmdReadADC(ctx, args[1:], out)
	case "read-battery":
		return cmdReadBattery(ctx, out)
	case "read-grayscale":
		return cmdReadGrayscale(ctx, out)
	case "distance":
		return cmdDistance(ctx, args[1:], out)
	case "-h", "--help", "help":
		printUsage(out)
		return nil
	default:
		printUsage(out)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// openPX opens the PiCar-X with calibration loaded from the example's store.
func openPX(ctx context.Context) (*picarx.PiCarX, error) {
	cal, _, err := loadCalibration()
	if err != nil {
		return nil, err
	}
	return picarx.Open(ctx, picarx.Options{
		I2CDev:      i2cPath,
		GPIOChip:    gpioChip,
		Calibration: cal,
	})
}

func cmdPingMCU(ctx context.Context, out io.Writer) error {
	px, err := openPX(ctx)
	if err != nil {
		return err
	}
	defer px.Close()
	maj, min, pat, err := px.FirmwareVersion(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "MCU at %#x, firmware %d.%d.%d\n", px.Addr(), maj, min, pat)
	return nil
}

func cmdHATInfo(ctx context.Context, out io.Writer) error {
	px, err := openPX(ctx)
	if err != nil {
		return err
	}
	defer px.Close()
	h := px.HAT()
	fmt.Fprintf(out, "HAT %s\n  speaker-EN pin: %s\n  motor mode: %d\n  uuid: %s\n",
		h.Version, h.SpeakerENPin, h.MotorMode, h.UUID)
	return nil
}

func cmdBlink(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("blink", flag.ContinueOnError)
	fs.SetOutput(out)
	pin := fs.String("pin", "D14", "HAT pin name to blink")
	count := fs.Int("count", 5, "number of blinks")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Blink needs only GPIO — no MCU — so open the chip directly.
	chip, err := gpio.Open(gpioChip)
	if err != nil {
		return err
	}
	defer chip.Close()
	p, err := chip.RequestOutput(*pin, false)
	if err != nil {
		return err
	}
	defer p.Close()
	for i := 0; i < *count; i++ {
		if err := p.Write(true); err != nil {
			return err
		}
		time.Sleep(200 * time.Millisecond)
		if err := p.Write(false); err != nil {
			return err
		}
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Fprintf(out, "blinked %s %d times\n", *pin, *count)
	return nil
}

func cmdResetMCU(ctx context.Context, out io.Writer) error {
	px, err := openPX(ctx)
	if err != nil {
		return err
	}
	defer px.Close()
	if err := px.Reset(ctx); err != nil {
		return err
	}
	fmt.Fprintln(out, "MCU reset pulsed (GPIO5 low 10ms → high 10ms)")
	return nil
}
