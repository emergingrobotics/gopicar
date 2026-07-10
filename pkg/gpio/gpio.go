package gpio

import (
	"fmt"

	"github.com/warthog618/go-gpiocdev"
)

// Bias selects the internal pull resistor for an input line.
type Bias int

const (
	BiasAsIs Bias = iota // leave as configured (matches lgpio "no pull")
	BiasPullUp
	BiasPullDown
)

// Edge selects which line transitions produce events.
type Edge int

const (
	EdgeRising Edge = iota
	EdgeFalling
	EdgeBoth
)

// LineEvent is a kernel-timestamped edge event. TimestampNanos is monotonic on
// Linux ≥5.7, which is what makes the ultrasonic measurement accurate (§8).
type LineEvent struct {
	Offset         int
	Rising         bool
	TimestampNanos int64
}

// Pin is a single requested GPIO line.
type Pin interface {
	Write(v bool) error
	Read() (bool, error)
	Close() error
}

// Chip is a GPIO character device. internal/fake substitutes it in tests.
type Chip interface {
	RequestOutput(name string, initial bool) (Pin, error)
	RequestInput(name string, bias Bias) (Pin, error)
	RequestEdges(name string, edge Edge, bias Bias, handler func(LineEvent)) (Pin, error)
	Close() error
}

// cdevChip is the real /dev/gpiochip0 implementation over go-gpiocdev.
// NOTE: this layer needs real hardware to exercise; it is validated by the
// picarctl smoke test, not by unit tests. Verify the go-gpiocdev API against
// the installed version (v0.9.1) if it fails to build.
type cdevChip struct{ chip *gpiocdev.Chip }

// Open opens a GPIO chip by name, e.g. "gpiochip0".
func Open(name string) (Chip, error) {
	c, err := gpiocdev.NewChip(name)
	if err != nil {
		return nil, fmt.Errorf("gpio: open %s: %w", name, err)
	}
	return &cdevChip{c}, nil
}

func (c *cdevChip) Close() error { return c.chip.Close() }

type cdevPin struct{ line *gpiocdev.Line }

func (p *cdevPin) Write(v bool) error {
	n := 0
	if v {
		n = 1
	}
	return p.line.SetValue(n)
}

func (p *cdevPin) Read() (bool, error) {
	v, err := p.line.Value()
	return v != 0, err
}

func (p *cdevPin) Close() error { return p.line.Close() }

func (c *cdevChip) RequestOutput(name string, initial bool) (Pin, error) {
	off, err := ResolveOffset(name)
	if err != nil {
		return nil, err
	}
	iv := 0
	if initial {
		iv = 1
	}
	l, err := c.chip.RequestLine(off, gpiocdev.AsOutput(iv))
	if err != nil {
		return nil, fmt.Errorf("gpio: request output %s: %w", name, err)
	}
	return &cdevPin{l}, nil
}

func biasOption(bias Bias) []gpiocdev.LineReqOption {
	switch bias {
	case BiasPullUp:
		return []gpiocdev.LineReqOption{gpiocdev.WithPullUp}
	case BiasPullDown:
		return []gpiocdev.LineReqOption{gpiocdev.WithPullDown}
	default:
		return nil
	}
}

func (c *cdevChip) RequestInput(name string, bias Bias) (Pin, error) {
	off, err := ResolveOffset(name)
	if err != nil {
		return nil, err
	}
	opts := append([]gpiocdev.LineReqOption{gpiocdev.AsInput}, biasOption(bias)...)
	l, err := c.chip.RequestLine(off, opts...)
	if err != nil {
		return nil, fmt.Errorf("gpio: request input %s: %w", name, err)
	}
	return &cdevPin{l}, nil
}

func (c *cdevChip) RequestEdges(name string, edge Edge, bias Bias, handler func(LineEvent)) (Pin, error) {
	off, err := ResolveOffset(name)
	if err != nil {
		return nil, err
	}
	var edgeOpt gpiocdev.LineReqOption
	switch edge {
	case EdgeRising:
		edgeOpt = gpiocdev.WithRisingEdge
	case EdgeFalling:
		edgeOpt = gpiocdev.WithFallingEdge
	default:
		edgeOpt = gpiocdev.WithBothEdges
	}
	opts := []gpiocdev.LineReqOption{gpiocdev.AsInput, edgeOpt}
	opts = append(opts, biasOption(bias)...)
	opts = append(opts, gpiocdev.WithEventHandler(func(ev gpiocdev.LineEvent) {
		handler(LineEvent{
			Offset:         ev.Offset,
			Rising:         ev.Type == gpiocdev.LineEventRisingEdge,
			TimestampNanos: ev.Timestamp.Nanoseconds(),
		})
	}))
	l, err := c.chip.RequestLine(off, opts...)
	if err != nil {
		return nil, fmt.Errorf("gpio: request edges %s: %w", name, err)
	}
	return &cdevPin{l}, nil
}
