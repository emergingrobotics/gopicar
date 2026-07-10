package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunNoArgsPrintsUsage(t *testing.T) {
	var buf bytes.Buffer
	if err := run(nil, &buf); err != nil {
		t.Fatalf("run(nil) error: %v", err)
	}
	if !strings.Contains(buf.String(), "picarctl") {
		t.Fatalf("usage missing 'picarctl': %q", buf.String())
	}
	for _, cmd := range []string{"ping-mcu", "hat-info", "blink", "reset-mcu"} {
		if !strings.Contains(buf.String(), cmd) {
			t.Errorf("usage missing %q", cmd)
		}
	}
}

func TestRunUnknownCommandErrors(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"bogus"}, &buf); err == nil {
		t.Fatal("expected error for unknown command")
	}
}
