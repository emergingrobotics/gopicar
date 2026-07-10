// Package adc reads the Robot HAT MCU's 12-bit analog inputs: raw channel
// reads, voltage conversion, the battery divider, and the three grayscale
// line-sensor channels. It is usable standalone against any pkg/bus.Bus and is
// the sensor layer beneath pkg/picarx.
package adc

import (
	"context"
	"fmt"

	"github.com/emergingrobotics/gopicar/pkg/bus"
	"github.com/emergingrobotics/gopicar/pkg/mcu"
)

// Reference voltage and full-scale count for the 12-bit ADC (§5.2).
const (
	VRef      = 3.3
	FullScale = 4095.0
	// BatteryDivider is the multiplier applied to the A4 voltage to recover the
	// pack voltage (20k/10k divider) (§5.2).
	BatteryDivider = 3.0
)

// ADC reads analog channels from one MCU at a fixed I²C address.
type ADC struct {
	b    bus.Bus
	addr uint8
}

// New returns an ADC bound to bus b and MCU address addr.
func New(b bus.Bus, addr uint8) *ADC { return &ADC{b: b, addr: addr} }

// Read returns the raw 12-bit value (0..4095) of ADC channel chn (0..4).
//
// It reproduces the stock robot_hat sequence exactly: write the 3-byte frame
// [reg, 0, 0] as its OWN transaction (with a STOP), then read the two result
// bytes as SEPARATE single-byte reads, big-endian (§5.2).
//
// This firmware (HAT V4, fw 2.1.1) does NOT return valid data from a combined
// write+repeated-START read: that path yields the MSB but leaves the second
// byte 0x00 (so every reading looks like raw&0xFF00), and the missing STOP
// means the conversion is never latched. Verified on hardware against the
// reference library, which read a correct battery voltage where the combined
// read returned garbage.
func (a *ADC) Read(ctx context.Context, chn uint8) (int, error) {
	reg := mcu.ADCRegister(chn)
	// Transaction 1: S addr W reg 0x00 0x00 P
	if err := a.b.WriteBlock(ctx, a.addr, reg, []byte{0, 0}); err != nil {
		return 0, err
	}
	// Transactions 2 & 3: two bare single-byte reads (Tx with no write payload
	// issues a pure read: S addr R <byte> NACK P).
	msb, err := a.b.Tx(ctx, a.addr, nil, 1)
	if err != nil {
		return 0, err
	}
	lsb, err := a.b.Tx(ctx, a.addr, nil, 1)
	if err != nil {
		return 0, err
	}
	if len(msb) < 1 || len(lsb) < 1 {
		return 0, fmt.Errorf("adc: short read: msb=%d lsb=%d bytes", len(msb), len(lsb))
	}
	return int(msb[0])<<8 | int(lsb[0]), nil
}

// Voltage returns channel chn converted to volts on the 3.3 V reference.
func (a *ADC) Voltage(ctx context.Context, chn uint8) (float64, error) {
	raw, err := a.Read(ctx, chn)
	if err != nil {
		return 0, err
	}
	return RawToVoltage(raw), nil
}

// Battery returns the pack voltage: A4 voltage × 3 (§5.2).
func (a *ADC) Battery(ctx context.Context) (float64, error) {
	v, err := a.Voltage(ctx, 4)
	if err != nil {
		return 0, err
	}
	return v * BatteryDivider, nil
}

// Grayscale returns the three line-sensor channels A0/A1/A2 as [L, M, R] (§9).
func (a *ADC) Grayscale(ctx context.Context) ([3]int, error) {
	var out [3]int
	for i := uint8(0); i < 3; i++ {
		v, err := a.Read(ctx, i)
		if err != nil {
			return out, err
		}
		out[i] = v
	}
	return out, nil
}

// RawToVoltage converts a raw 12-bit reading to volts on the 3.3 V reference.
func RawToVoltage(raw int) float64 { return float64(raw) * VRef / FullScale }
