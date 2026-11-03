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
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/profilerpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg := config.FromEnv()
	address := env("BLOG_PROFILE_GRPC_ADDRESS", ":9098")
	logger := logging.New("blog-profile")
	repo, err := repository.Open(cfg.DatabaseURL)
	if err != nil {
		logger.Error("open profile database", "error", err)
		os.Exit(1)
	}
	if err := repo.ConfigurePool(cfg.DBMaxOpenConns, cfg.DBMaxIdleConns, cfg.DBConnMaxLifetime); err != nil {
		logger.Error("configure profile database pool", "error", err)
		os.Exit(1)
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		logger.Error("listen for profile RPC", "address", address, "error", err)
		os.Exit(1)
	}
	server := profilerpc.NewGRPCServer(logger)
	profilerpc.RegisterServer(server, profilerpc.NewServer(repo))
	reflection.Register(server)
	logger.Info("profile RPC listening", "address", address)
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.Serve(listener) }()
	shutdown, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverErrors:
		if err != nil {
			logger.Error("profile RPC stopped", "error", err)
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
