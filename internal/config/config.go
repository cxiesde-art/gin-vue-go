package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Address                  string
	DatabaseURL              string
	JWTSecret                string
	AdminEmail               string
	AdminPass                string
	ContentGRPCAddress       string
	ContentGRPCRetryAttempts int
	ContentGRPCRetryDelay    time.Duration
	MaxBodyBytes             int64
	RateLimitRequests        int
	RateLimitWindow          time.Duration
}

func FromEnv() Config {
	return Config{
		Address:                  env("BLOG_ADDRESS", ":8080"),
		DatabaseURL:              env("BLOG_DATABASE_URL", "blog.db"),
		JWTSecret:                env("BLOG_JWT_SECRET", "development-secret-change-me"),
		AdminEmail:               env("BLOG_ADMIN_EMAIL", "admin@example.com"),
		AdminPass:                env("BLOG_ADMIN_PASSWORD", "admin123"),
		ContentGRPCAddress:       env("BLOG_CONTENT_GRPC_ADDRESS", ""),
		ContentGRPCRetryAttempts: envInt("BLOG_CONTENT_GRPC_RETRY_ATTEMPTS", 3),
		ContentGRPCRetryDelay:    envDuration("BLOG_CONTENT_GRPC_RETRY_DELAY", 50*time.Millisecond),
		MaxBodyBytes:             envInt64("BLOG_MAX_BODY_BYTES", 1<<20),
		RateLimitRequests:        envInt("BLOG_RATE_LIMIT_REQUESTS", 120),
		RateLimitWindow:          envDuration("BLOG_RATE_LIMIT_WINDOW", time.Minute),
	}
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envInt64(key string, fallback int64) int64 {
	value, err := strconv.ParseInt(os.Getenv(key), 10, 64)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
