package bus

import (
	"context"
	"fmt"
)

// Probe tries addrs in order and returns the first that responds to a 1-byte
// read of reg (§5.1). Returns an error (never a silent fallback) if none do.
func Probe(ctx context.Context, b Bus, addrs []uint8, reg uint8) (uint8, error) {
	var lastErr error
	for _, a := range addrs {
		if _, err := b.ReadBlock(ctx, a, reg, 1); err == nil {
			return a, nil
		} else {
			lastErr = err
		}
	}
	return 0, fmt.Errorf("bus: no device responded on %v: %w", addrs, lastErr)
}
