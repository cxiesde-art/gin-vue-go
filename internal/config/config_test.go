package config

import (
	"testing"
	"time"
)

func TestFromEnvParsesDistributedSettings(t *testing.T) {
	t.Setenv("BLOG_CONTENT_GRPC_ADDRESS", "content:9090")
	t.Setenv("BLOG_CONTENT_GRPC_RETRY_ATTEMPTS", "5")
	t.Setenv("BLOG_CONTENT_GRPC_RETRY_DELAY", "125ms")
	cfg := FromEnv()
	if cfg.ContentGRPCAddress != "content:9090" || cfg.ContentGRPCRetryAttempts != 5 || cfg.ContentGRPCRetryDelay != 125*time.Millisecond {
		t.Fatalf("unexpected distributed config: %#v", cfg)
	}
}

func TestFromEnvUsesSafeDefaultsForInvalidValues(t *testing.T) {
	t.Setenv("BLOG_CONTENT_GRPC_RETRY_ATTEMPTS", "invalid")
	t.Setenv("BLOG_CONTENT_GRPC_RETRY_DELAY", "invalid")
	cfg := FromEnv()
	if cfg.ContentGRPCRetryAttempts != 3 || cfg.ContentGRPCRetryDelay != 50*time.Millisecond {
		t.Fatalf("unexpected defaults: attempts=%d delay=%s", cfg.ContentGRPCRetryAttempts, cfg.ContentGRPCRetryDelay)
	}
}
