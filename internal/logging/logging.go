package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"time"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/metrics"
	"github.com/gin-gonic/gin"
)

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	traceIDKey   contextKey = "trace_id"
)

func New(service string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})).With("service", service)
}

// NewText returns a human-readable logger for local development.
func NewText(service string) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})).With("service", service)
}

// WithIDs adds distributed tracing identifiers to a request context.
func WithIDs(ctx context.Context, requestID, traceID string) context.Context {
	ctx = context.WithValue(ctx, requestIDKey, requestID)
	return context.WithValue(ctx, traceIDKey, traceID)
}

func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func TraceID(ctx context.Context) string {
	value, _ := ctx.Value(traceIDKey).(string)
	return value
}

func HTTPMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return HTTPMiddlewareWithMetrics(logger, nil)
}

func HTTPMiddlewareWithMetrics(logger *slog.Logger, registry *metrics.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = NewID()
		}
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = requestID
		}
		c.Request = c.Request.WithContext(WithIDs(c.Request.Context(), requestID, traceID))
		c.Header("X-Request-ID", requestID)
		c.Header("X-Trace-ID", traceID)
		c.Next()
		logger.InfoContext(c.Request.Context(), "http_request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(started).Milliseconds(),
		)
		if registry != nil {
			registry.ObserveHTTP(c.Writer.Status(), time.Since(started))
		}
	}
}

func NewID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(bytes)
}
