package contentrpc

import (
	"testing"
	"time"
)

func TestCircuitBreakerOpensAndAllowsOneProbe(t *testing.T) {
	now := time.Unix(100, 0)
	breaker := NewCircuitBreaker(2, time.Second)
	breaker.Record(now, ErrCircuitOpen)
	if !breaker.Allow(now) {
		t.Fatal("breaker blocked before threshold")
	}
	breaker.Record(now, ErrCircuitOpen)
	if breaker.Allow(now) {
		t.Fatal("breaker allowed call while open")
	}
	probeTime := now.Add(time.Second)
	if !breaker.Allow(probeTime) || breaker.Allow(probeTime) {
		t.Fatal("breaker did not enforce a single half-open probe")
	}
	breaker.Record(probeTime, nil)
	if !breaker.Allow(probeTime) {
		t.Fatal("successful probe did not close breaker")
	}
}
