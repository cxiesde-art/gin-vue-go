package contentrpc

import (
	"errors"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("content service circuit is open")

type CircuitBreaker struct {
	mu           sync.Mutex
	failures     int
	threshold    int
	resetAfter   time.Duration
	openedAt     time.Time
	halfOpenUsed bool
}

func NewCircuitBreaker(threshold int, resetAfter time.Duration) *CircuitBreaker {
	if threshold < 1 {
		threshold = 5
	}
	if resetAfter <= 0 {
		resetAfter = 5 * time.Second
	}
	return &CircuitBreaker{threshold: threshold, resetAfter: resetAfter}
}

func (b *CircuitBreaker) Allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.openedAt.IsZero() {
		return true
	}
	if now.Sub(b.openedAt) < b.resetAfter {
		return false
	}
	if b.halfOpenUsed {
		return false
	}
	b.halfOpenUsed = true
	return true
}

func (b *CircuitBreaker) Record(now time.Time, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err == nil {
		b.failures = 0
		b.openedAt = time.Time{}
		b.halfOpenUsed = false
		return
	}
	b.failures++
	if b.failures >= b.threshold {
		b.openedAt = now
		b.halfOpenUsed = false
	}
}
