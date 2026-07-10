package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// parsePWMChannel accepts "P12", "p12", or "12" and returns the channel number.
func parsePWMChannel(s string) (uint8, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "P"), "p")
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 19 {
		return 0, fmt.Errorf("invalid PWM channel %q (want P0..P19)", s)
	}
	return uint8(n), nil
}

// parseADCChannel accepts "A0".."A4" or "0".."4".
func parseADCChannel(s string) (uint8, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "A"), "a")
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 4 {
		return 0, fmt.Errorf("invalid ADC channel %q (want A0..A4)", s)
	}
	return uint8(n), nil
}

func clampFloat(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
