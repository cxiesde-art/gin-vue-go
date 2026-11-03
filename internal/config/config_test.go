package config

import (
	"testing"
	"time"
)

func TestFromEnvParsesDistributedSettings(t *testing.T) {
	t.Setenv("BLOG_CONTENT_GRPC_ADDRESS", "content:9090")
	t.Setenv("BLOG_ANALYTICS_GRPC_ADDRESS", "analytics:9093")
	t.Setenv("BLOG_TAXONOMY_GRPC_ADDRESS", "taxonomy:9094")
	t.Setenv("BLOG_SEARCH_GRPC_ADDRESS", "search:9095")
	t.Setenv("BLOG_AUDIT_GRPC_ADDRESS", "audit:9096")
	t.Setenv("BLOG_POST_GRPC_ADDRESS", "posts:9097")
	t.Setenv("BLOG_CONTENT_GRPC_RETRY_ATTEMPTS", "5")
	t.Setenv("BLOG_CONTENT_GRPC_RETRY_DELAY", "125ms")
	cfg := FromEnv()
	if cfg.ContentGRPCAddress != "content:9090" || cfg.AnalyticsGRPCAddress != "analytics:9093" || cfg.TaxonomyGRPCAddress != "taxonomy:9094" || cfg.SearchGRPCAddress != "search:9095" || cfg.AuditGRPCAddress != "audit:9096" || cfg.PostGRPCAddress != "posts:9097" || cfg.ContentGRPCRetryAttempts != 5 || cfg.ContentGRPCRetryDelay != 125*time.Millisecond {
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

func TestFromEnvParsesTrustedProxies(t *testing.T) {
	t.Setenv("BLOG_TRUSTED_PROXIES", "10.0.0.0/8, 192.0.2.4")
	cfg := FromEnv()
	if len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[0] != "10.0.0.0/8" || cfg.TrustedProxies[1] != "192.0.2.4" {
		t.Fatalf("unexpected trusted proxies: %#v", cfg.TrustedProxies)
	}
}
