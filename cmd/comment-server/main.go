package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/config"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/logging"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/commentrpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg := config.FromEnv()
	address := env("BLOG_COMMENT_GRPC_ADDRESS", ":9092")
	logger := logging.New("blog-comments")
	repo, err := repository.Open(cfg.DatabaseURL)
	if err != nil {
		logger.Error("open comments database", "error", err)
		os.Exit(1)
	}
	if err := repo.ConfigurePool(cfg.DBMaxOpenConns, cfg.DBMaxIdleConns, cfg.DBConnMaxLifetime); err != nil {
		logger.Error("configure comments database pool", "error", err)
		os.Exit(1)
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		logger.Error("listen for comments RPC", "address", address, "error", err)
		os.Exit(1)
	}
	server := commentrpc.NewGRPCServer(logger)
	commentrpc.RegisterServer(server, commentrpc.NewServer(repo))
	reflection.Register(server)
	logger.Info("comments RPC listening", "address", address)
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.Serve(listener) }()
	shutdown, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverErrors:
		if err != nil {
			logger.Error("comments RPC stopped", "error", err)
			os.Exit(1)
		}
	case <-shutdown.Done():
		server.GracefulStop()
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
