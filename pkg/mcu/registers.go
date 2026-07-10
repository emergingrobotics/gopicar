// Package mcu holds the Robot HAT MCU register map, I²C addresses, HAT
// detection, and the reset sequence. Every other package imports register
// constants from here so there is a single source of truth (§5.3, §15).
package mcu

// ProbeAddrs are the 7-bit I²C slave addresses probed in order (§5.1).
var ProbeAddrs = []uint8{0x14, 0x15, 0x16}

// MCU register map (§5.3, §15).
const (
	RegPWMChanBase   uint8 = 0x20 // pulse-width "on value" for channel n: base + n
	RegTimerPSCBase  uint8 = 0x40 // timer prescaler, timers 0..3: base + t
	RegTimerARRBase  uint8 = 0x44 // timer period (ARR), timers 0..3: base + t
	RegTimerPSC2Base uint8 = 0x50 // V5 timers 4..6 prescaler
	RegTimerARR2Base uint8 = 0x54 // V5 timers 4..6 period
	RegFirmwareVer   uint8 = 0x05 // firmware version, 3 bytes major/minor/patch (§15)
	RegADCBattery    uint8 = 0x13 // ADC channel 4, battery divider (§5.2)
)

// PWMInputClock is the PWM peripheral input clock in Hz (§5.3).
const PWMInputClock = 72_000_000

// ADCRegister returns the read register for ADC channel n (0..4): (7-n)|0x10 (§5.2).
func ADCRegister(n uint8) uint8 { return (7 - n) | 0x10 }
