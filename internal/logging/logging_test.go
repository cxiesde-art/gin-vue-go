package logging

import (
	"context"
	"testing"
)

func TestDistributedIDsRoundTrip(t *testing.T) {
	ctx := WithIDs(context.Background(), "request-1", "trace-1")
	if RequestID(ctx) != "request-1" || TraceID(ctx) != "trace-1" {
		t.Fatalf("unexpected IDs: request=%q trace=%q", RequestID(ctx), TraceID(ctx))
	}
	if len(NewID()) != 32 {
		t.Fatal("generated ID should contain 16 random bytes")
	}
}
