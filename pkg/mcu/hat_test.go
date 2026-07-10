package mcu_test

import (
	"testing"

	"github.com/emergingrobotics/gopicar/internal/fake"
	"github.com/emergingrobotics/gopicar/pkg/mcu"
)

func TestDetectHATv5(t *testing.T) {
	root := t.TempDir()
	if err := fake.WriteDeviceTree(root, "9daeea78-0000-076e-0032-582369ac3e02"); err != nil {
		t.Fatal(err)
	}
	h := mcu.DetectHAT(root)
	if h.Version != mcu.HATv5 || h.SpeakerENPin != "D10" || h.MotorMode != 2 {
		t.Fatalf("got %+v, want V5/D10/mode2", h)
	}
}

func TestDetectHATv4Fallback(t *testing.T) {
	root := t.TempDir() // no device-tree at all
	h := mcu.DetectHAT(root)
	if h.Version != mcu.HATv4 || h.SpeakerENPin != "D15" || h.MotorMode != 1 {
		t.Fatalf("got %+v, want V4/D15/mode1", h)
	}
}

func TestDetectHATUnknownUUIDIsV4(t *testing.T) {
	root := t.TempDir()
	if err := fake.WriteDeviceTree(root, "deadbeef-0000-0000-0000-000000000000"); err != nil {
		t.Fatal(err)
	}
	if h := mcu.DetectHAT(root); h.Version != mcu.HATv4 {
		t.Fatalf("unknown uuid should be V4, got %v", h.Version)
	}
}
