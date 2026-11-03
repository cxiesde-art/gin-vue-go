package contentrpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRetryRPCEventuallySucceeds(t *testing.T) {
	attempts := 0
	err := retryRPC(context.Background(), RetryPolicy{MaxAttempts: 3, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond}, func() error {
		attempts++
		if attempts < 3 {
			return status.Error(codes.Unavailable, "content service warming up")
		}
		return nil
	})
	if err != nil || attempts != 3 {
		t.Fatalf("retry did not recover: attempts=%d error=%v", attempts, err)
	}
}

func TestRetryRPCStopsAtLimitAndDoesNotRetryPermanentErrors(t *testing.T) {
	attempts := 0
	permanent := errors.New("invalid query")
	err := retryRPC(context.Background(), RetryPolicy{MaxAttempts: 4, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond}, func() error {
		attempts++
		return permanent
	})
	if !errors.Is(err, permanent) || attempts != 1 {
		t.Fatalf("permanent error was retried: attempts=%d error=%v", attempts, err)
	}

	attempts = 0
	err = retryRPC(context.Background(), RetryPolicy{MaxAttempts: 2, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond}, func() error {
		attempts++
		return status.Error(codes.Unavailable, "still unavailable")
	})
	if status.Code(err) != codes.Unavailable || attempts != 2 {
		t.Fatalf("retry limit was ignored: attempts=%d error=%v", attempts, err)
	}
}
