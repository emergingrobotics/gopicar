//go:build hardware

// Package picarx hardware integration tests. These run only with the
// `hardware` build tag on a Raspberry Pi with a Robot HAT attached:
//
//	go test -tags hardware ./pkg/picarx/
//
// Tests that physically move actuators additionally require GOPICAR_HW_MOVE=1
// so a bare `-tags hardware` run stays non-destructive.
package picarx_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/emergingrobotics/gopicar/pkg/picarx"
)

func openHW(t *testing.T) *picarx.PiCarX {
	t.Helper()
	px, err := picarx.Open(context.Background(), picarx.Options{Calibration: picarx.MeasuredCalibration()})
	if err != nil {
		t.Fatalf("Open (is the HAT attached?): %v", err)
	}
	return px
}

func TestHWFirmware(t *testing.T) {
	px := openHW(t)
	defer px.Close()
	maj, min, pat, err := px.FirmwareVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if maj == 0 && min == 0 && pat == 0 {
		t.Error("firmware version reads 0.0.0 — MCU not responding")
	}
	t.Logf("MCU %#x firmware %d.%d.%d", px.Addr(), maj, min, pat)
}

func TestHWBatteryPlausible(t *testing.T) {
	px := openHW(t)
	defer px.Close()
	v, err := px.Battery(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("battery = %.2f V", v)
	if v < 5.5 || v > 8.6 {
		t.Errorf("battery %.2f V outside plausible 5.5–8.6 V band (bad ADC read?)", v)
	}
}

func TestHWServoSweep(t *testing.T) {
	if os.Getenv("GOPICAR_HW_MOVE") != "1" {
		t.Skip("set GOPICAR_HW_MOVE=1 to run actuator-moving tests")
	}
	px := openHW(t)
	defer px.Close()
	ctx := context.Background()
	for _, a := range []float64{-20, 0, 20, 0} {
		if err := px.SetDir(ctx, a); err != nil {
			t.Fatal(err)
		}
		time.Sleep(400 * time.Millisecond)
	}
}
