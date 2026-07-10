package fake

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/emergingrobotics/gopicar/pkg/gpio"
)

func TestBusRecordsAndResponds(t *testing.T) {
	b := NewBus()
	b.Responses[Key(0x14, 0x17)] = []byte{0x0A, 0x0B}
	got, err := b.ReadBlock(context.Background(), 0x14, 0x17, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []byte{0x0A, 0x0B}) {
		t.Fatalf("read=%v want [0x0A 0x0B]", got)
	}
	if len(b.Txns) != 1 || b.Txns[0].Addr != 0x14 {
		t.Fatalf("txns=%+v", b.Txns)
	}
}

func TestBusFailFirst(t *testing.T) {
	b := NewBus()
	b.FailFirst = 2
	ctx := context.Background()
	if err := b.WriteRawByte(ctx, 0x14, 0x00); err == nil {
		t.Fatal("call 1 should fail")
	}
	if err := b.WriteRawByte(ctx, 0x14, 0x00); err == nil {
		t.Fatal("call 2 should fail")
	}
	if err := b.WriteRawByte(ctx, 0x14, 0x00); err != nil {
		t.Fatalf("call 3 should succeed, got %v", err)
	}
}

func TestBusOnlyAddr(t *testing.T) {
	b := NewBus()
	a := uint8(0x15)
	b.OnlyAddr = &a
	if err := b.WriteRawByte(context.Background(), 0x14, 0); err == nil {
		t.Fatal("wrong addr should error")
	}
	if err := b.WriteRawByte(context.Background(), 0x15, 0); err != nil {
		t.Fatalf("right addr should succeed: %v", err)
	}
}

func TestChipWritesAndEdge(t *testing.T) {
	c := NewChip()
	pin, err := c.RequestOutput("MCURST", true)
	if err != nil {
		t.Fatal(err)
	}
	_ = pin.Write(false)
	_ = pin.Write(true)
	if !reflect.DeepEqual(c.Pins["MCURST"].Writes, []bool{false, true}) {
		t.Fatalf("writes=%v", c.Pins["MCURST"].Writes)
	}

	var gotTS int64
	var gotRising bool
	_, err = c.RequestEdges("D3", gpio.EdgeBoth, gpio.BiasPullDown, func(e gpio.LineEvent) {
		gotTS, gotRising = e.TimestampNanos, e.Rising
	})
	if err != nil {
		t.Fatal(err)
	}
	c.InjectEdge("D3", true, 42)
	if gotTS != 42 || !gotRising {
		t.Fatalf("edge not delivered: ts=%d rising=%v", gotTS, gotRising)
	}
}

func TestChipRequestUnknownPinErrors(t *testing.T) {
	c := NewChip()
	if _, err := c.RequestOutput("NOPE", false); err == nil {
		t.Fatal("expected error for unknown pin name")
	}
}

func TestWriteDeviceTree(t *testing.T) {
	root := t.TempDir()
	if err := WriteDeviceTree(root, "abc-123"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "proc", "device-tree", "hat", "uuid"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "abc-123\x00" {
		t.Fatalf("uuid file = %q", string(data))
	}
}
