package gpio

import "fmt"

// nameToOffset maps Robot HAT pin labels and aliases to BCM GPIO offsets on
// gpiochip0 (§3.1). On the Pi, gpiochip0 line offsets equal BCM GPIO numbers.
var nameToOffset = map[string]int{
	"D0": 17, "D1": 4, "D2": 27, "D3": 22, "D4": 23, "D5": 24, "D6": 25, "D7": 4,
	"D8": 5, "D9": 6, "D10": 12, "D11": 13, "D12": 19, "D13": 16, "D14": 26, "D15": 20, "D16": 21,
	"SW": 25, "USER": 25, "LED": 26, "BOARD_TYPE": 12, "RST": 16,
	"BLEINT": 13, "BLERST": 20, "MCURST": 5, "CE": 8,
}

// ResolveOffset returns the gpiochip0 line offset for a HAT pin name.
func ResolveOffset(name string) (int, error) {
	if off, ok := nameToOffset[name]; ok {
		return off, nil
	}
	return 0, fmt.Errorf("gpio: unknown pin name %q", name)
}
