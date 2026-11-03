package searchrpc

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
			return status.Error(codes.Unavailable, "search service warming up")
		}
		return nil
	})
	if err != nil || attempts != 3 {
		t.Fatalf("retry did not recover: attempts=%d error=%v", attempts, err)
	}
}

func TestRetryRPCDoesNotRetryPermanentErrors(t *testing.T) {
	attempts := 0
	permanent := errors.New("invalid query")
	err := retryRPC(context.Background(), RetryPolicy{MaxAttempts: 4, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond}, func() error {
		attempts++
		return permanent
	})
	if !errors.Is(err, permanent) || attempts != 1 {
		t.Fatalf("permanent error was retried: attempts=%d error=%v", attempts, err)
	}
}
