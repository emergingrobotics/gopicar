package picarx_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/emergingrobotics/gopicar/pkg/picarx"
)

// These examples show real usage. They have no "// Output:" line, so `go test`
// compiles them (guaranteeing the API stays valid) but does not execute them —
// they would need real hardware.

func Example() {
	ctx := context.Background()
	px, err := picarx.Open(ctx, picarx.Options{Calibration: picarx.MeasuredCalibration()})
	if err != nil {
		log.Fatal(err)
	}
	defer px.Close()

	px.SetDir(ctx, 0)     // steering straight
	px.SetCamPan(ctx, 0)  // camera forward
	px.SetCamTilt(ctx, 0) // camera level

	v, _ := px.Battery(ctx)
	fmt.Printf("battery: %.2f V\n", v)

	px.Forward(ctx, 40)
	time.Sleep(time.Second)
	px.Stop(ctx)
}

func ExamplePiCarX_Ramp() {
	ctx := context.Background()
	px, err := picarx.Open(ctx, picarx.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer px.Close()

	// Ease from a standstill to 60% forward over 1.5 s, then stop.
	px.Ramp(ctx, 0, 60, 1500*time.Millisecond)
	px.Stop(ctx)
}

func ExamplePiCarX_Distance() {
	ctx := context.Background()
	px, err := picarx.Open(ctx, picarx.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer px.Close()

	cm, err := px.Distance(ctx, 20*time.Millisecond)
	if err != nil {
		log.Printf("no echo: %v", err)
		return
	}
	fmt.Printf("%.1f cm ahead\n", cm)
}
