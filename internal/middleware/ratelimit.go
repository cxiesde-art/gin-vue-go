package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]window
	limit   int
	period  time.Duration
}

type window struct {
	started time.Time
	count   int
}

func NewRateLimiter(limit int, period time.Duration) *RateLimiter {
	if limit < 1 {
		limit = 60
	}
	if period <= 0 {
		period = time.Minute
	}
	return &RateLimiter{clients: make(map[string]window), limit: limit, period: period}
}

func (r *RateLimiter) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := clientKey(c)
		now := time.Now()
		r.mu.Lock()
		current := r.clients[key]
		if current.started.IsZero() || now.Sub(current.started) >= r.period {
			current = window{started: now, count: 0}
		}
		current.count++
		r.clients[key] = current
		allowed := current.count <= r.limit
		remaining := r.limit - current.count
		if remaining < 0 {
			remaining = 0
		}
		r.mu.Unlock()

		c.Header("X-RateLimit-Limit", formatInt(r.limit))
		c.Header("X-RateLimit-Remaining", formatInt(remaining))
		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

func clientKey(c *gin.Context) string {
	if forwarded := c.GetHeader("X-Forwarded-For"); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	return c.ClientIP()
}

func formatInt(value int) string {
	return strconv.Itoa(value)
}
