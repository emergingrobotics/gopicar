package bus

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestWriteBlockFraming(t *testing.T) {
	var gotAddr uint8
	var gotW []byte
	var gotRead int
	orig := rdwr
	defer func() { rdwr = orig }()
	rdwr = func(fd int, addr uint8, w []byte, readN int) ([]byte, error) {
		gotAddr, gotW, gotRead = addr, append([]byte(nil), w...), readN
		return nil, nil
	}

	b := &I2C{fd: -1}
	if err := b.WriteBlock(context.Background(), 0x14, 0x20, []byte{0x08, 0x00}); err != nil {
		t.Fatal(err)
	}
	if gotAddr != 0x14 || !reflect.DeepEqual(gotW, []byte{0x20, 0x08, 0x00}) || gotRead != 0 {
		t.Fatalf("addr=%#x w=%v readN=%d; want 0x14 [0x20 0x08 0x00] 0", gotAddr, gotW, gotRead)
	}
}

func TestReadBlockFraming(t *testing.T) {
	var gotW []byte
	var gotRead int
	orig := rdwr
	defer func() { rdwr = orig }()
	rdwr = func(fd int, addr uint8, w []byte, readN int) ([]byte, error) {
		gotW, gotRead = append([]byte(nil), w...), readN
		return []byte{0xAB, 0xCD}, nil
	}

	b := &I2C{fd: -1}
	got, err := b.ReadBlock(context.Background(), 0x14, 0x17, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotW, []byte{0x17}) || gotRead != 2 {
		t.Fatalf("w=%v readN=%d; want [0x17] 2", gotW, gotRead)
	}
	if !reflect.DeepEqual(got, []byte{0xAB, 0xCD}) {
		t.Fatalf("read=%v; want [0xAB 0xCD]", got)
	}
}

func TestXferHonorsCancelledContext(t *testing.T) {
	called := false
	orig := rdwr
	defer func() { rdwr = orig }()
	rdwr = func(fd int, addr uint8, w []byte, readN int) ([]byte, error) {
		called = true
		return nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b := &I2C{fd: -1}
	err := b.WriteRawByte(ctx, 0x14, 0x00)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v; want context.Canceled", err)
	}
	if called {
		t.Fatal("rdwr should not be called when ctx is already cancelled")
	}
}
