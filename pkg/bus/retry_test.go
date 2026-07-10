package bus

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// stubBus fails the first failN calls with err, then succeeds.
type stubBus struct {
	calls int
	failN int
	err   error
}

func (s *stubBus) hit() error {
	s.calls++
	if s.calls <= s.failN {
		return s.err
	}
	return nil
}
func (s *stubBus) WriteBlock(ctx context.Context, addr, reg uint8, data []byte) error {
	return s.hit()
}
func (s *stubBus) ReadBlock(ctx context.Context, addr, reg uint8, n int) ([]byte, error) {
	return nil, s.hit()
}
func (s *stubBus) WriteRawByte(ctx context.Context, addr, v uint8) error { return s.hit() }
func (s *stubBus) Tx(ctx context.Context, addr uint8, w []byte, readN int) ([]byte, error) {
	return nil, s.hit()
}

func newTestRetry(inner Bus, attempts int) *retryBus {
	return &retryBus{
		inner: inner,
		p:     RetryPolicy{Attempts: attempts, BaseDelay: 0},
		sleep: func(context.Context, time.Duration) error { return nil }, // no real delay
	}
}

func TestRetrySucceedsAfterTransientErrors(t *testing.T) {
	s := &stubBus{failN: 2, err: unix.EIO}
	r := newTestRetry(s, 5)
	if err := r.WriteBlock(context.Background(), 0x14, 0x20, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.calls != 3 {
		t.Fatalf("calls = %d, want 3", s.calls)
	}
}

func TestRetryExhaustsAndReturnsError(t *testing.T) {
	s := &stubBus{failN: 99, err: unix.EIO}
	r := newTestRetry(s, 5)
	err := r.WriteRawByte(context.Background(), 0x14, 0x00)
	if !errors.Is(err, unix.EIO) {
		t.Fatalf("err = %v, want EIO", err)
	}
	if s.calls != 5 {
		t.Fatalf("calls = %d, want 5", s.calls)
	}
}

func TestRetryDoesNotRetryNonRetryable(t *testing.T) {
	s := &stubBus{failN: 99, err: unix.EINVAL}
	r := newTestRetry(s, 5)
	if err := r.WriteRawByte(context.Background(), 0x14, 0x00); !errors.Is(err, unix.EINVAL) {
		t.Fatalf("err = %v, want EINVAL", err)
	}
	if s.calls != 1 {
		t.Fatalf("calls = %d, want 1 (no retry on non-retryable)", s.calls)
	}
}

func TestRetryAbortsOnContextCancel(t *testing.T) {
	s := &stubBus{failN: 99, err: unix.EIO}
	r := &retryBus{
		inner: s,
		p:     RetryPolicy{Attempts: 5, BaseDelay: time.Millisecond},
		sleep: func(context.Context, time.Duration) error { return context.Canceled },
	}
	err := r.WriteRawByte(context.Background(), 0x14, 0x00)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if s.calls != 1 {
		t.Fatalf("calls = %d, want 1 (aborted during backoff)", s.calls)
	}
}

func TestCtxSleepReturnsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled: the select must take ctx.Done() immediately
	if err := ctxSleep(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("ctxSleep = %v, want context.Canceled", err)
	}
}

func TestCtxSleepCompletesWhenTimerFires(t *testing.T) {
	if err := ctxSleep(context.Background(), time.Millisecond); err != nil {
		t.Fatalf("ctxSleep = %v, want nil", err)
	}
}
