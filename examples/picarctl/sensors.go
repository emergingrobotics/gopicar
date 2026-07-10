package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/emergingrobotics/gopicar/pkg/adc"
)

func cmdReadADC(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("read-adc", flag.ContinueOnError)
	fs.SetOutput(out)
	channel := fs.String("channel", "A0", "ADC channel A0..A4")
	if err := fs.Parse(args); err != nil {
		return err
	}
	chn, err := parseADCChannel(*channel)
	if err != nil {
		return err
	}
	px, err := openPX(ctx)
	if err != nil {
		return err
	}
	defer px.Close()
	raw, err := px.ADC().Read(ctx, chn)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "A%d: raw=%d  voltage=%.3f V\n", chn, raw, adc.RawToVoltage(raw))
	return nil
}

func cmdReadBattery(ctx context.Context, out io.Writer) error {
	px, err := openPX(ctx)
	if err != nil {
		return err
	}
	defer px.Close()
	v, err := px.Battery(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "battery: %.2f V\n", v)
	return nil
}

func cmdReadGrayscale(ctx context.Context, out io.Writer) error {
	px, err := openPX(ctx)
	if err != nil {
		return err
	}
	defer px.Close()
	g, err := px.Grayscale(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "grayscale [L M R] = [%d %d %d]\n", g[0], g[1], g[2])
	return nil
}
