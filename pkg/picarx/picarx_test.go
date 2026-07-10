package picarx_test

import (
	"context"
	"testing"

	"github.com/emergingrobotics/gopicar/internal/fake"
	"github.com/emergingrobotics/gopicar/pkg/mcu"
	"github.com/emergingrobotics/gopicar/pkg/picarx"
)

// openFake builds a PiCarX over a fully faked bus+chip so tests need no hardware.
func openFake(t *testing.T) (*picarx.PiCarX, *fake.Bus) {
	t.Helper()
	b := fake.NewBus()
	b.Responses[fake.Key(0x14, mcu.RegFirmwareVer)] = []byte{2, 1, 1} // fw 2.1.1
	chip := fake.NewChip()
	px, err := picarx.Open(context.Background(), picarx.Options{
		Bus:         b,
		Chip:        chip,
		Calibration: picarx.MeasuredCalibration(),
		MCU:         mcu.Options{DeviceTreeRoot: t.TempDir()},
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return px, b
}

func TestOpenAndFirmware(t *testing.T) {
	px, _ := openFake(t)
	defer px.Close()
	if px.Addr() != 0x14 {
		t.Errorf("Addr = %#x, want 0x14", px.Addr())
	}
	maj, min, pat, err := px.FirmwareVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if maj != 2 || min != 1 || pat != 1 {
		t.Errorf("firmware = %d.%d.%d, want 2.1.1", maj, min, pat)
	}
}

func TestSetDirAppliesCalibration(t *testing.T) {
	px, b := openFake(t)
	defer px.Close()
	if err := px.SetDir(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	// steer center = raw -58° on channel 2 (reg 0x22): duty 0x00AF.
	last := b.Txns[len(b.Txns)-1].Write
	if last[0] != 0x22 || last[1] != 0x00 || last[2] != 0xAF {
		t.Errorf("SetDir(0) last write = %v, want [0x22 0x00 0xAF]", last)
	}
}

func TestBattery(t *testing.T) {
	px, b := openFake(t)
	defer px.Close()
	b.EnqueueRead(0x14, 0x0D, 0x68) // 3432 → 8.30 V
	v, err := px.Battery(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v < 8.2 || v > 8.4 {
		t.Errorf("Battery = %.2f, want ~8.3", v)
	}
}

func TestForwardDrivesBothMotors(t *testing.T) {
	px, b := openFake(t)
	defer px.Close()
	before := len(b.Txns)
	if err := px.Forward(context.Background(), 40); err != nil {
		t.Fatal(err)
	}
	// Two motor duty writes (one per motor) appended.
	if got := len(b.Txns) - before; got != 2 {
		t.Errorf("Forward wrote %d motor txns, want 2", got)
	}
}

func TestLineAndCliffStatus(t *testing.T) {
	px, b := openFake(t)
	defer px.Close()
	ref := [3]int{500, 500, 500}
	// All three below ref → line on all, cliff true. Grayscale reads 3 channels.
	b.EnqueueRead(0x14, 0x00, 0x64, 0x00, 0x64, 0x00, 0x64) // 100,100,100
	ls, err := px.LineStatus(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if ls != [3]bool{true, true, true} {
		t.Errorf("LineStatus = %v, want all true", ls)
	}
	b.EnqueueRead(0x14, 0x00, 0x64, 0x00, 0x64, 0x00, 0x64)
	cliff, err := px.CliffStatus(context.Background(), ref)
	if err != nil || !cliff {
		t.Errorf("CliffStatus = %v (err %v), want true", cliff, err)
	}
}

func TestCloseIdempotent(t *testing.T) {
	px, _ := openFake(t)
	if err := px.Close(); err != nil {
		t.Fatal(err)
	}
	if err := px.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}
