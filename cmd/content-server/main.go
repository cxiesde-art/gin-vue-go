package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/config"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/logging"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/metrics"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/contentrpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg := config.FromEnv()
	address := env("BLOG_CONTENT_GRPC_ADDRESS", ":9090")
	logger := logging.New("blog-content")
	repo, err := repository.Open(cfg.DatabaseURL)
	if err != nil {
		logger.Error("open content database", "error", err)
		os.Exit(1)
	}
	if err := repo.ConfigurePool(cfg.DBMaxOpenConns, cfg.DBMaxIdleConns, cfg.DBConnMaxLifetime); err != nil {
		logger.Error("configure content database pool", "error", err)
		os.Exit(1)
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		logger.Error("listen for content RPC", "address", address, "error", err)
		os.Exit(1)
	}
	server := contentrpc.NewGRPCServerWithMetrics(logger, metrics.New())
	contentrpc.RegisterServer(server, contentrpc.NewServer(repo))
	reflection.Register(server)
	logger.InfoContext(context.Background(), "content_rpc_listening", "address", address)
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.Serve(listener) }()
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-serverErrors:
		if err != nil {
			logger.Error("content RPC stopped", "error", err)
			os.Exit(1)
		}
	case signal := <-shutdown:
		logger.Info("content RPC shutdown signal received", "signal", signal.String())
		server.GracefulStop()
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
