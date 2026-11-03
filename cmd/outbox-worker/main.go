package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/config"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/logging"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/outbox"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
)

func main() {
	cfg := config.FromEnv()
	logger := logging.New("blog-outbox")
	repo, err := repository.Open(cfg.DatabaseURL)
	if err != nil {
		logger.Error("open outbox database", "error", err)
		os.Exit(1)
	}
	if err := repo.ConfigurePool(cfg.DBMaxOpenConns, cfg.DBMaxIdleConns, cfg.DBConnMaxLifetime); err != nil {
		logger.Error("configure outbox database pool", "error", err)
		os.Exit(1)
	}
	interval := durationEnv("BLOG_OUTBOX_INTERVAL", time.Second)
	worker := outbox.NewWorker(repo, outbox.NewLogPublisher(logger), logger, interval, 100)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := worker.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("outbox worker stopped", "error", err)
		os.Exit(1)
	}
	logger.Info("outbox worker stopped")
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		if seconds, parseErr := strconv.Atoi(value); parseErr == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
		return fallback
	}
	if parsed <= 0 {
		return fallback
	}
	return parsed
}
