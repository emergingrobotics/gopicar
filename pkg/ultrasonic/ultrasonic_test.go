package ultrasonic_test

import (
	"context"
	"testing"
	"time"

	"github.com/emergingrobotics/gopicar/internal/fake"
	"github.com/emergingrobotics/gopicar/pkg/ultrasonic"
)

func TestDistanceFromEchoPulse(t *testing.T) {
	chip := fake.NewChip()
	s := ultrasonic.New(chip, "", "") // defaults D2/D3

	// A 1 ms echo pulse → 0.001 * 343.3 / 2 * 100 = 17.165 cm.
	go func() {
		time.Sleep(5 * time.Millisecond) // let Distance arm the edge handler
		chip.InjectEdge("D3", true, 0)
		chip.InjectEdge("D3", false, 1_000_000) // 1 ms later
	}()

	cm, err := s.Distance(context.Background(), 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if cm < 17.0 || cm > 17.3 {
		t.Fatalf("distance = %.3f cm, want ~17.165", cm)
	}
}

func TestDistanceTimeout(t *testing.T) {
	chip := fake.NewChip()
	s := ultrasonic.New(chip, "", "")
	_, err := s.Distance(context.Background(), 20*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error with no echo injected")
	}
}

func TestDistanceContextCancel(t *testing.T) {
	chip := fake.NewChip()
	s := ultrasonic.New(chip, "", "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Distance(ctx, time.Second); err == nil {
		t.Fatal("expected context cancellation error")
	}
}
