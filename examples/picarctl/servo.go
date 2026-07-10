package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/emergingrobotics/gopicar/pkg/pwm"
)

func cmdServo(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("servo", flag.ContinueOnError)
	fs.SetOutput(out)
	which := fs.String("which", "", "named servo: pan | tilt | steer (applies calibration + limits)")
	channel := fs.String("channel", "", "raw PWM channel, e.g. P0/P1/P2 (no calibration applied)")
	angle := fs.Float64("angle", 0, "target angle in degrees")
	raw := fs.Bool("raw", false, "with --which, ignore calibration and send --angle as the raw MCU angle")
	fs.Usage = func() {
		fmt.Fprintln(out, "Usage: picarctl servo (--which pan|tilt|steer | --channel P0) --angle N [--raw]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	px, err := openPX(ctx)
	if err != nil {
		return err
	}
	defer px.Close()

	switch {
	case *which != "":
		s := px.Servo(*which)
		if s == nil {
			return fmt.Errorf("unknown servo %q (want pan, tilt, or steer)", *which)
		}
		if *raw {
			if err := s.SetRaw(ctx, clampFloat(*angle, -90, 90)); err != nil {
				return err
			}
			fmt.Fprintf(out, "servo %s → raw P%d %.1f°\n", *which, s.Channel(), clampFloat(*angle, -90, 90))
			return nil
		}
		if err := s.Angle(ctx, *angle); err != nil {
			return err
		}
		cal := s.Calibration()
		fmt.Fprintf(out, "servo %s: user %.1f° → raw P%d %.1f° (trim %+.0f dir %+.0f)\n",
			*which, *angle, s.Channel(), cal.Apply(*angle), cal.Trim, cal.Dir)
		return nil

	case *channel != "":
		ch, err := parsePWMChannel(*channel)
		if err != nil {
			return err
		}
		target := clampFloat(*angle, -90, 90)
		if err := pwm.SetServoAngle(ctx, px.Bus(), px.Addr(), ch, target); err != nil {
			return err
		}
		fmt.Fprintf(out, "servo P%d → %.1f° (raw)\n", ch, target)
		return nil

	default:
		fs.Usage()
		return fmt.Errorf("specify --which or --channel")
	}
}
