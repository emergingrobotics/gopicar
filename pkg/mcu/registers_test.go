package mcu

import "testing"

func TestADCRegister(t *testing.T) {
	// §5.2: channel n → (7-n)|0x10, so 0→0x17 … 4→0x13.
	want := map[uint8]uint8{0: 0x17, 1: 0x16, 2: 0x15, 3: 0x14, 4: 0x13}
	for n, w := range want {
		if got := ADCRegister(n); got != w {
			t.Errorf("ADCRegister(%d) = %#x, want %#x", n, got, w)
		}
	}
}

func TestClockConstant(t *testing.T) {
	if PWMInputClock != 72_000_000 {
		t.Errorf("PWMInputClock = %d, want 72000000", PWMInputClock)
	}
}
