package bus

import (
	"context"
	"errors"
	"time"

	"golang.org/x/sys/unix"
)

// RetryPolicy controls the retry decorator.
type RetryPolicy struct {
	Attempts  int
	BaseDelay time.Duration
}

// DefaultRetryPolicy matches the Python reference: 5 attempts, exp backoff from
// 1 ms (§5.1) — but the final error is returned, never swallowed (§17).
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{Attempts: 5, BaseDelay: time.Millisecond}
}

type retryBus struct {
	inner Bus
	p     RetryPolicy
	sleep func(context.Context, time.Duration) error
}

// WithRetry wraps a Bus with the given retry policy.
func WithRetry(inner Bus, p RetryPolicy) Bus {
	return &retryBus{inner: inner, p: p, sleep: ctxSleep}
}

func ctxSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func retryable(err error) bool {
	return errors.Is(err, unix.EIO) || errors.Is(err, unix.ENXIO)
}

func (r *retryBus) do(ctx context.Context, op func() error) error {
	delay := r.p.BaseDelay
	var err error
	for attempt := 0; attempt < r.p.Attempts; attempt++ {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		err = op()
		if err == nil || !retryable(err) {
			return err
		}
		if attempt < r.p.Attempts-1 {
			if serr := r.sleep(ctx, delay); serr != nil {
				return serr
			}
			delay *= 2
		}
	}
	return err
}

func (r *retryBus) WriteBlock(ctx context.Context, addr, reg uint8, data []byte) error {
	return r.do(ctx, func() error { return r.inner.WriteBlock(ctx, addr, reg, data) })
}

func (r *retryBus) ReadBlock(ctx context.Context, addr, reg uint8, n int) ([]byte, error) {
	var out []byte
	err := r.do(ctx, func() error {
		var e error
		out, e = r.inner.ReadBlock(ctx, addr, reg, n)
		return e
	})
	return out, err
}

func (r *retryBus) WriteRawByte(ctx context.Context, addr, v uint8) error {
	return r.do(ctx, func() error { return r.inner.WriteRawByte(ctx, addr, v) })
}

func (r *retryBus) Tx(ctx context.Context, addr uint8, w []byte, readN int) ([]byte, error) {
	var out []byte
	err := r.do(ctx, func() error {
		var e error
		out, e = r.inner.Tx(ctx, addr, w, readN)
		return e
	})
	return out, err
}

var _ Bus = (*retryBus)(nil)
