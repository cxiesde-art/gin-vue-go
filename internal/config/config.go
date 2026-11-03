package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address                  string
	DatabaseURL              string
	JWTSecret                string
	AdminEmail               string
	AdminPass                string
	ContentGRPCAddress       string
	IdentityGRPCAddress      string
	CommentGRPCAddress       string
	ProfileGRPCAddress       string
	AnalyticsGRPCAddress     string
	TaxonomyGRPCAddress      string
	SearchGRPCAddress        string
	AuditGRPCAddress         string
	PostGRPCAddress          string
	ContentGRPCRetryAttempts int
	ContentGRPCRetryDelay    time.Duration
	MaxBodyBytes             int64
	RateLimitRequests        int
	RateLimitWindow          time.Duration
	TrustedProxies           []string
	DBMaxOpenConns           int
	DBMaxIdleConns           int
	DBConnMaxLifetime        time.Duration
}

func FromEnv() Config {
	return Config{
		Address:                  env("BLOG_ADDRESS", ":8080"),
		DatabaseURL:              env("BLOG_DATABASE_URL", "blog.db"),
		JWTSecret:                env("BLOG_JWT_SECRET", "development-secret-change-me"),
		AdminEmail:               env("BLOG_ADMIN_EMAIL", "admin@example.com"),
		AdminPass:                env("BLOG_ADMIN_PASSWORD", "admin123"),
		ContentGRPCAddress:       env("BLOG_CONTENT_GRPC_ADDRESS", ""),
		IdentityGRPCAddress:      env("BLOG_IDENTITY_GRPC_ADDRESS", ""),
		CommentGRPCAddress:       env("BLOG_COMMENT_GRPC_ADDRESS", ""),
		ProfileGRPCAddress:       env("BLOG_PROFILE_GRPC_ADDRESS", ""),
		AnalyticsGRPCAddress:     env("BLOG_ANALYTICS_GRPC_ADDRESS", ""),
		TaxonomyGRPCAddress:      env("BLOG_TAXONOMY_GRPC_ADDRESS", ""),
		SearchGRPCAddress:        env("BLOG_SEARCH_GRPC_ADDRESS", ""),
		AuditGRPCAddress:         env("BLOG_AUDIT_GRPC_ADDRESS", ""),
		PostGRPCAddress:          env("BLOG_POST_GRPC_ADDRESS", ""),
		ContentGRPCRetryAttempts: envInt("BLOG_CONTENT_GRPC_RETRY_ATTEMPTS", 3),
		ContentGRPCRetryDelay:    envDuration("BLOG_CONTENT_GRPC_RETRY_DELAY", 50*time.Millisecond),
		MaxBodyBytes:             envInt64("BLOG_MAX_BODY_BYTES", 1<<20),
		RateLimitRequests:        envInt("BLOG_RATE_LIMIT_REQUESTS", 120),
		RateLimitWindow:          envDuration("BLOG_RATE_LIMIT_WINDOW", time.Minute),
		TrustedProxies:           envList("BLOG_TRUSTED_PROXIES"),
		DBMaxOpenConns:           envInt("BLOG_DB_MAX_OPEN_CONNS", 10),
		DBMaxIdleConns:           envInt("BLOG_DB_MAX_IDLE_CONNS", 5),
		DBConnMaxLifetime:        envDuration("BLOG_DB_CONN_MAX_LIFETIME", time.Hour),
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

func envList(key string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	return values
}
