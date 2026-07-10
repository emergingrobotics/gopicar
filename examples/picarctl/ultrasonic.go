package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/emergingrobotics/gopicar/pkg/gpio"
	"github.com/emergingrobotics/gopicar/pkg/ultrasonic"
)

func cmdDistance(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("distance", flag.ContinueOnError)
	fs.SetOutput(out)
	count := fs.Int("count", 1, "number of measurements")
	timeout := fs.Duration("timeout", 20*time.Millisecond, "per-measurement echo timeout (§8)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Ultrasonic uses only GPIO — no I²C — so it works even if the MCU is dead.
	chip, err := gpio.Open(gpioChip)
	if err != nil {
		return err
	}
	defer chip.Close()
	sensor := ultrasonic.New(chip, "", "")

	for i := 0; i < *count; i++ {
		cm, err := sensor.Distance(ctx, *timeout)
		if err != nil {
			fmt.Fprintf(out, "distance: %v\n", err)
		} else {
			fmt.Fprintf(out, "distance: %.2f cm\n", cm)
		}
		if i < *count-1 {
			time.Sleep(60 * time.Millisecond) // let echoes die out between pings
		}
	}
	return nil
}
