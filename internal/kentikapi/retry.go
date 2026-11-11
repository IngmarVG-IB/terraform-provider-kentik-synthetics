package kentikapi

import (
	"context"
	"math/rand/v2"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Retry defaults.
const (
	DefaultMaxAttempts = 5
	DefaultMinDelay    = time.Second
	DefaultMaxDelay    = 30 * time.Second
)

// RetryConfig configures retries with exponential backoff and full jitter.
type RetryConfig struct {
	// MaxAttempts is the total number of attempts, including the first one. Zero means DefaultMaxAttempts;
	// 1 disables retries.
	MaxAttempts int
	// MinDelay is the base delay before the first retry. Zero means DefaultMinDelay.
	MinDelay time.Duration
	// MaxDelay caps the delay between attempts. Zero means DefaultMaxDelay.
	MaxDelay time.Duration
	// Codes are the status codes that are retried. Empty means Unavailable and ResourceExhausted.
	Codes []codes.Code
}

func (r RetryConfig) withDefaults() RetryConfig {
	if r.MaxAttempts <= 0 {
		r.MaxAttempts = DefaultMaxAttempts
	}
	if r.MinDelay <= 0 {
		r.MinDelay = DefaultMinDelay
	}
	if r.MaxDelay <= 0 {
		r.MaxDelay = DefaultMaxDelay
	}
	if r.MaxDelay < r.MinDelay {
		r.MaxDelay = r.MinDelay
	}
	if len(r.Codes) == 0 {
		r.Codes = []codes.Code{codes.Unavailable, codes.ResourceExhausted}
	}
	return r
}

func (r RetryConfig) retryable(err error) bool {
	c := status.Code(err)
	for _, rc := range r.Codes {
		if c == rc {
			return true
		}
	}
	return false
}

// backoff returns the delay before retry number n (n >= 1).
func (r RetryConfig) backoff(n int) time.Duration {
	d := r.MinDelay
	for i := 1; i < n && d < r.MaxDelay; i++ {
		d *= 2
	}
	d = min(d, r.MaxDelay)
	// Full jitter in [d/2, d) spreads retries of concurrent Terraform operations.
	half := d / 2
	return half + time.Duration(rand.Int64N(int64(half)+1)) //nolint:gosec // jitter, not security sensitive
}

// sleep is replaced in tests.
var sleep = func(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func retryInterceptor(r RetryConfig) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context, method string, req, reply any, cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
	) error {
		var err error
		for attempt := 1; ; attempt++ {
			err = invoker(ctx, method, req, reply, cc, opts...)
			if err == nil || attempt >= r.MaxAttempts || !r.retryable(err) {
				return err
			}
			if serr := sleep(ctx, r.backoff(attempt)); serr != nil {
				return err
			}
		}
	}
}
