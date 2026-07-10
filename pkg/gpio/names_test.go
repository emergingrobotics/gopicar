package gpio

import "testing"

func TestResolveOffset(t *testing.T) {
	// §3.1 mapping + aliases.
	want := map[string]int{
		"D2": 27, "D3": 22, "D4": 23, "D5": 24, "D14": 26,
		"MCURST": 5, "LED": 26, "SW": 25, "RST": 16, "CE": 8,
	}
	for name, w := range want {
		got, err := ResolveOffset(name)
		if err != nil {
			t.Errorf("ResolveOffset(%q) error: %v", name, err)
			continue
		}
		if got != w {
			t.Errorf("ResolveOffset(%q) = %d, want %d", name, got, w)
		}
	}
}

func TestResolveOffsetUnknown(t *testing.T) {
	if _, err := ResolveOffset("NOPE"); err == nil {
		t.Fatal("expected error for unknown pin name")
	}
}
