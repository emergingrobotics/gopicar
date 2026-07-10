package adc_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/emergingrobotics/gopicar/internal/fake"
	"github.com/emergingrobotics/gopicar/pkg/adc"
)

func TestReadTransactionShape(t *testing.T) {
	b := fake.NewBus()
	// A0 → reg (7-0)|0x10 = 0x17. Queue MSB=0x0D, LSB=0x66 → 0x0D66 = 3430.
	b.EnqueueRead(0x14, 0x0D, 0x66)
	a := adc.New(b, 0x14)

	got, err := a.Read(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0x0D66 {
		t.Fatalf("Read = %d, want %d", got, 0x0D66)
	}
	// Assert exact transaction shape: write frame + two bare 1-byte reads.
	if len(b.Txns) != 3 {
		t.Fatalf("got %d txns, want 3: %+v", len(b.Txns), b.Txns)
	}
	if !reflect.DeepEqual(b.Txns[0].Write, []byte{0x17, 0x00, 0x00}) {
		t.Errorf("txn0 write = %v, want [0x17 0 0]", b.Txns[0].Write)
	}
	for i := 1; i <= 2; i++ {
		if len(b.Txns[i].Write) != 0 || len(b.Txns[i].Read) != 1 {
			t.Errorf("txn%d = %+v; want bare 1-byte read", i, b.Txns[i])
		}
	}
}

func TestChannelRegisters(t *testing.T) {
	// Each channel must write its (7-n)|0x10 register byte.
	regs := map[uint8]byte{0: 0x17, 1: 0x16, 2: 0x15, 3: 0x14, 4: 0x13}
	for ch, reg := range regs {
		b := fake.NewBus()
		b.EnqueueRead(0x14, 0x00, 0x00)
		if _, err := adc.New(b, 0x14).Read(context.Background(), ch); err != nil {
			t.Fatal(err)
		}
		if b.Txns[0].Write[0] != reg {
			t.Errorf("ch %d wrote reg %#x, want %#x", ch, b.Txns[0].Write[0], reg)
		}
	}
}

func TestBattery(t *testing.T) {
	b := fake.NewBus()
	// A4 raw 3432 → 2.766 V → ×3 = 8.30 V (matches the measured hardware value).
	b.EnqueueRead(0x14, 0x0D, 0x68) // 0x0D68 = 3432
	v, err := adc.New(b, 0x14).Battery(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v < 8.2 || v > 8.4 {
		t.Fatalf("Battery = %.3f V, want ~8.3 V", v)
	}
}

func TestGrayscale(t *testing.T) {
	b := fake.NewBus()
	// Three reads (A0,A1,A2), MSB/LSB each.
	b.EnqueueRead(0x14, 0x00, 0x5D, 0x00, 0x65, 0x00, 0x57) // 93, 101, 87
	g, err := adc.New(b, 0x14).Grayscale(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if g != [3]int{93, 101, 87} {
		t.Fatalf("Grayscale = %v, want [93 101 87]", g)
	}
}
