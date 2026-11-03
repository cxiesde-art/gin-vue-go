package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/auth"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/config"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/handler"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/logging"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/contentrpc"
)

func main() {
	cfg := config.FromEnv()
	logger := logging.New("blog-api")
	repo, err := repository.Open(cfg.DatabaseURL)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	if err := repo.SeedAdmin(context.Background(), cfg.AdminEmail, auth.HashPassword(cfg.AdminPass)); err != nil {
		logger.Error("seed administrator", "error", err)
		os.Exit(1)
	}

	var reader handler.ContentReader
	var contentClient *contentrpc.Client
	if cfg.ContentGRPCAddress != "" {
		dialContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		contentClient, err = contentrpc.DialWithPolicy(dialContext, cfg.ContentGRPCAddress, logger, contentrpc.RetryPolicy{
			MaxAttempts:  cfg.ContentGRPCRetryAttempts,
			InitialDelay: cfg.ContentGRPCRetryDelay,
		})
		cancel()
		if err != nil {
			logger.Error("connect to content RPC", "address", cfg.ContentGRPCAddress, "error", err)
			os.Exit(1)
		}
		defer contentClient.Close()
		reader = contentClient
		logger.Info("using distributed content service", "address", cfg.ContentGRPCAddress)
	}

	server := &http.Server{Addr: cfg.Address, Handler: handler.NewWithReader(cfg, repo, reader, logger).Router()}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	logger.Info("blog API listening", "address", cfg.Address, "database", cfg.DatabaseURL)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("serve HTTP", "error", err)
			os.Exit(1)
		}
	case signal := <-shutdown:
		logger.Info("shutdown signal received", "signal", signal.String())
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			logger.Error("shutdown HTTP", "error", err)
			os.Exit(1)
		}
	}
}
