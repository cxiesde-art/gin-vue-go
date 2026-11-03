package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type RateLimiter struct {
	mu             sync.Mutex
	clients        map[string]window
	limit          int
	period         time.Duration
	trustedProxies []*net.IPNet
}

type window struct {
	started time.Time
	count   int
}

func NewRateLimiter(limit int, period time.Duration) *RateLimiter {
	return NewRateLimiterWithTrustedProxies(limit, period, nil)
}

func NewRateLimiterWithTrustedProxies(limit int, period time.Duration, proxies []string) *RateLimiter {
	if limit < 1 {
		limit = 60
	}
	if period <= 0 {
		period = time.Minute
	}
	trustedProxies := make([]*net.IPNet, 0, len(proxies))
	for _, proxy := range proxies {
		if ip := net.ParseIP(strings.TrimSpace(proxy)); ip != nil {
			bits := 128
			if ip4 := ip.To4(); ip4 != nil {
				ip = ip4
				bits = 32
			}
			trustedProxies = append(trustedProxies, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		if _, network, err := net.ParseCIDR(strings.TrimSpace(proxy)); err == nil {
			trustedProxies = append(trustedProxies, network)
		}
	}
	return &RateLimiter{clients: make(map[string]window), limit: limit, period: period, trustedProxies: trustedProxies}
}

func (r *RateLimiter) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := clientKey(c, r.trustedProxies)
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
		resetAfter := time.Until(current.started.Add(r.period))
		r.mu.Unlock()

		c.Header("X-RateLimit-Limit", formatInt(r.limit))
		c.Header("X-RateLimit-Remaining", formatInt(remaining))
		if !allowed {
			seconds := int(resetAfter / time.Second)
			if resetAfter%time.Second != 0 {
				seconds++
			}
			if seconds < 1 {
				seconds = 1
			}
			c.Header("Retry-After", strconv.Itoa(seconds))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

func clientKey(c *gin.Context, trustedProxies []*net.IPNet) string {
	remote := strings.TrimSpace(c.Request.RemoteAddr)
	host, _, err := net.SplitHostPort(remote)
	if err == nil {
		remote = host
	}
	if ip := net.ParseIP(remote); ip != nil {
		for _, proxy := range trustedProxies {
			if proxy.Contains(ip) {
				forwarded := strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-For"), ",")[0])
				if clientIP := net.ParseIP(forwarded); clientIP != nil {
					return clientIP.String()
				}
				break
			}
		}
	}
	if remote == "" {
		return "unknown"
	}
	return remote
}

func formatInt(value int) string {
	return strconv.Itoa(value)
}
