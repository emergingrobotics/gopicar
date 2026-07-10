package fake

import "github.com/emergingrobotics/gopicar/pkg/gpio"

// Chip is an in-memory gpio.Chip.
type Chip struct {
	Pins map[string]*Pin
}

func NewChip() *Chip { return &Chip{Pins: map[string]*Pin{}} }

// Pin is an in-memory gpio.Pin recording its write history.
type Pin struct {
	Name   string
	Offset int
	Value  bool
	Writes []bool
	edge   func(gpio.LineEvent)
}

func (c *Chip) add(name string, initial bool, handler func(gpio.LineEvent)) (*Pin, error) {
	off, err := gpio.ResolveOffset(name)
	if err != nil {
		return nil, err
	}
	p := &Pin{Name: name, Offset: off, Value: initial, edge: handler}
	c.Pins[name] = p
	return p, nil
}

func (c *Chip) RequestOutput(name string, initial bool) (gpio.Pin, error) {
	p, err := c.add(name, initial, nil)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (c *Chip) RequestInput(name string, bias gpio.Bias) (gpio.Pin, error) {
	p, err := c.add(name, false, nil)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (c *Chip) RequestEdges(name string, edge gpio.Edge, bias gpio.Bias, handler func(gpio.LineEvent)) (gpio.Pin, error) {
	p, err := c.add(name, false, handler)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (c *Chip) Close() error { return nil }

// InjectEdge delivers a synthetic edge to a pin requested via RequestEdges.
func (c *Chip) InjectEdge(name string, rising bool, tsNanos int64) {
	if p := c.Pins[name]; p != nil && p.edge != nil {
		p.edge(gpio.LineEvent{Offset: p.Offset, Rising: rising, TimestampNanos: tsNanos})
	}
}

func (p *Pin) Write(v bool) error {
	p.Value = v
	p.Writes = append(p.Writes, v)
	return nil
}

func (p *Pin) Read() (bool, error) { return p.Value, nil }
func (p *Pin) Close() error        { return nil }

var (
	_ gpio.Chip = (*Chip)(nil)
	_ gpio.Pin  = (*Pin)(nil)
)
