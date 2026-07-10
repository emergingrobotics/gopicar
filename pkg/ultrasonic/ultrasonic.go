// Package ultrasonic measures distance with the PiCar-X HC-SR04 sensor. It
// fires a trigger pulse and times the echo using kernel-timestamped GPIO edges
// — far steadier than a busy-wait (§8). It needs only GPIO, so it works even if
// the MCU is unresponsive.
package ultrasonic

import (
	"context"
	"fmt"
	"time"

	"github.com/emergingrobotics/gopicar/pkg/gpio"
)

// Default trigger/echo HAT pins (§8).
const (
	DefaultTrigPin = "D2" // GPIO27, output
	DefaultEchoPin = "D3" // GPIO22, input, pull-down
)

// SpeedOfSound is used to convert echo round-trip time to distance (m/s).
const SpeedOfSound = 343.3

// Sensor is an HC-SR04 on a trigger + echo GPIO pin pair.
type Sensor struct {
	chip    gpio.Chip
	trigPin string
	echoPin string
}

// New returns a Sensor. Pass empty strings to use DefaultTrigPin/DefaultEchoPin.
func New(chip gpio.Chip, trigPin, echoPin string) *Sensor {
	if trigPin == "" {
		trigPin = DefaultTrigPin
	}
	if echoPin == "" {
		echoPin = DefaultEchoPin
	}
	return &Sensor{chip: chip, trigPin: trigPin, echoPin: echoPin}
}

// Distance fires one ping and returns the distance in centimeters, or an error
// on timeout / context cancellation. timeout bounds how long to wait for the
// echo (a proxy for maximum range).
func (s *Sensor) Distance(ctx context.Context, timeout time.Duration) (float64, error) {
	trig, err := s.chip.RequestOutput(s.trigPin, false)
	if err != nil {
		return 0, err
	}
	defer trig.Close()

	events := make(chan gpio.LineEvent, 8)
	echo, err := s.chip.RequestEdges(s.echoPin, gpio.EdgeBoth, gpio.BiasPullDown, func(e gpio.LineEvent) {
		select {
		case events <- e:
		default:
		}
	})
	if err != nil {
		return 0, err
	}
	defer echo.Close()

	// Trigger: ≥10 µs high pulse after settling low (§8).
	if err := trig.Write(false); err != nil {
		return 0, err
	}
	time.Sleep(2 * time.Millisecond)
	if err := trig.Write(true); err != nil {
		return 0, err
	}
	time.Sleep(10 * time.Microsecond)
	if err := trig.Write(false); err != nil {
		return 0, err
	}

	deadline := time.After(timeout)
	var riseTS int64
	haveRise := false
	for {
		select {
		case e := <-events:
			if e.Rising {
				riseTS = e.TimestampNanos
				haveRise = true
			} else if haveRise {
				return echoNanosToCm(e.TimestampNanos - riseTS), nil
			}
		case <-deadline:
			return 0, fmt.Errorf("ultrasonic: echo timeout (>%.0f cm or no return)", echoNanosToCm(timeout.Nanoseconds()))
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
}

// echoNanosToCm converts an echo round-trip time in nanoseconds to centimeters.
func echoNanosToCm(dtNanos int64) float64 {
	return float64(dtNanos) * 1e-9 * SpeedOfSound / 2 * 100
}
