package main

import (
	"context"
	"log/slog"
	"net"
	"os"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/config"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/logging"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/metrics"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/contentrpc"
)

func main() {
	cfg := config.FromEnv()
	address := env("BLOG_CONTENT_GRPC_ADDRESS", ":9090")
	repo, err := repository.Open(cfg.DatabaseURL)
	if err != nil {
		slog.Error("open content database", "error", err)
		os.Exit(1)
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		slog.Error("listen for content RPC", "address", address, "error", err)
		os.Exit(1)
	}
	logger := logging.New("blog-content")
	server := contentrpc.NewGRPCServerWithMetrics(logger, metrics.New())
	contentrpc.RegisterServer(server, contentrpc.NewServer(repo))
	logger.InfoContext(context.Background(), "content_rpc_listening", "address", address)
	if err := server.Serve(listener); err != nil {
		logger.Error("content RPC stopped", "error", err)
		os.Exit(1)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
