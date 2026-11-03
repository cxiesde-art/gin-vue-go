package contentrpc

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RetryPolicy struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
}

var DefaultRetryPolicy = RetryPolicy{MaxAttempts: 3, InitialDelay: 50 * time.Millisecond, MaxDelay: 500 * time.Millisecond}

func (p RetryPolicy) normalize() RetryPolicy {
	if p.MaxAttempts < 1 {
		p.MaxAttempts = DefaultRetryPolicy.MaxAttempts
	}
	if p.InitialDelay <= 0 {
		p.InitialDelay = DefaultRetryPolicy.InitialDelay
	}
	if p.MaxDelay < p.InitialDelay {
		p.MaxDelay = DefaultRetryPolicy.MaxDelay
	}
	return p
}

func retryRPC(ctx context.Context, policy RetryPolicy, operation func() error) error {
	policy = policy.normalize()
	delay := policy.InitialDelay
	var err error
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		err = operation()
		if err == nil || !retryable(err) || attempt == policy.MaxAttempts {
			return err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		delay *= 2
		if delay > policy.MaxDelay {
			delay = policy.MaxDelay
		}
	}
	return err
}

func retryable(err error) bool {
	code := status.Code(err)
	return code == codes.Unavailable || code == codes.DeadlineExceeded || code == codes.ResourceExhausted
}
