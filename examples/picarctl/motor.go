package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"
)

func cmdMotor(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("motor", flag.ContinueOnError)
	fs.SetOutput(out)
	which := fs.String("which", "", "which motor: left | right")
	speed := fs.Float64("speed", 0, "speed -100..100 (sign sets direction)")
	raw := fs.Bool("raw", false, "write |speed| directly as PWM duty %% (skip the stock speed/2+50 remap)")
	dur := fs.Duration("duration", 1*time.Second, "run time before auto-stop (0 = leave running)")
	fs.Usage = func() {
		fmt.Fprintln(out, "Usage: picarctl motor --which left|right --speed -100..100 [--raw] [--duration 1s]")
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

	m := px.Motor(*which)
	if m == nil {
		fs.Usage()
		return fmt.Errorf("specify --which left or right")
	}
	if err := m.Speed(ctx, *speed, *raw); err != nil {
		return err
	}
	fmt.Fprintf(out, "motor %s → speed %.0f\n", *which, *speed)

	if *dur > 0 {
		select {
		case <-time.After(*dur):
		case <-ctx.Done():
			fmt.Fprintln(out, "\ninterrupted — stopping motor")
		}
		// Fresh context: if ctx was cancelled by Ctrl-C, a write on the
		// cancelled context would be rejected and the motor would keep
		// spinning. The stop must go out regardless.
		if err := m.Stop(context.Background()); err != nil {
			return err
		}
		fmt.Fprintf(out, "motor %s stopped\n", *which)
	}
	return nil
}

func cmdStop(ctx context.Context, out io.Writer) error {
	px, err := openPX(ctx)
	if err != nil {
		return err
	}
	defer px.Close()
	if err := px.Stop(ctx); err != nil {
		return err
	}
	fmt.Fprintln(out, "both motors stopped")
	return nil
}
