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
	"github.com/cxiesde-art/gin-vue-blog-go/internal/metrics"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/analyticsrpc"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/authrpc"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/commentrpc"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/contentrpc"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/searchrpc"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/transport/taxonomyrpc"
)

func main() {
	cfg := config.FromEnv()
	logger := logging.New("blog-api")
	repo, err := repository.Open(cfg.DatabaseURL)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	if err := repo.ConfigurePool(cfg.DBMaxOpenConns, cfg.DBMaxIdleConns, cfg.DBConnMaxLifetime); err != nil {
		logger.Error("configure database pool", "error", err)
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
	var identity handler.IdentityProvider
	var identityClient *authrpc.Client
	if cfg.IdentityGRPCAddress != "" {
		dialContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		identityClient, err = authrpc.Dial(dialContext, cfg.IdentityGRPCAddress, logger)
		cancel()
		if err != nil {
			logger.Error("connect to identity RPC", "address", cfg.IdentityGRPCAddress, "error", err)
			os.Exit(1)
		}
		defer identityClient.Close()
		identity = identityClient
		logger.Info("using distributed identity service", "address", cfg.IdentityGRPCAddress)
	}
	var comments handler.CommentProvider
	var commentClient *commentrpc.Client
	if cfg.CommentGRPCAddress != "" {
		dialContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		commentClient, err = commentrpc.Dial(dialContext, cfg.CommentGRPCAddress, logger)
		cancel()
		if err != nil {
			logger.Error("connect to comments RPC", "address", cfg.CommentGRPCAddress, "error", err)
			os.Exit(1)
		}
		defer commentClient.Close()
		comments = commentClient
		logger.Info("using distributed comments service", "address", cfg.CommentGRPCAddress)
	}
	var analytics handler.AnalyticsProvider
	var analyticsClient *analyticsrpc.Client
	if cfg.AnalyticsGRPCAddress != "" {
		dialContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		analyticsClient, err = analyticsrpc.Dial(dialContext, cfg.AnalyticsGRPCAddress, logger)
		cancel()
		if err != nil {
			logger.Error("connect to analytics RPC", "address", cfg.AnalyticsGRPCAddress, "error", err)
			os.Exit(1)
		}
		defer analyticsClient.Close()
		analytics = analyticsClient
		logger.Info("using distributed analytics service", "address", cfg.AnalyticsGRPCAddress)
	}
	var taxonomy handler.TaxonomyProvider
	var taxonomyClient *taxonomyrpc.Client
	if cfg.TaxonomyGRPCAddress != "" {
		dialContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		taxonomyClient, err = taxonomyrpc.Dial(dialContext, cfg.TaxonomyGRPCAddress, logger)
		cancel()
		if err != nil {
			logger.Error("connect to taxonomy RPC", "address", cfg.TaxonomyGRPCAddress, "error", err)
			os.Exit(1)
		}
		defer taxonomyClient.Close()
		taxonomy = taxonomyClient
		logger.Info("using distributed taxonomy service", "address", cfg.TaxonomyGRPCAddress)
	}
	var search handler.SearchProvider
	var searchClient *searchrpc.Client
	if cfg.SearchGRPCAddress != "" {
		dialContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		searchClient, err = searchrpc.Dial(dialContext, cfg.SearchGRPCAddress, logger)
		cancel()
		if err != nil {
			logger.Error("connect to search RPC", "address", cfg.SearchGRPCAddress, "error", err)
			os.Exit(1)
		}
		defer searchClient.Close()
		search = searchClient
		logger.Info("using distributed search service", "address", cfg.SearchGRPCAddress)
	}

	server := &http.Server{Addr: cfg.Address, Handler: handler.NewWithServices(cfg, repo, reader, identity, comments, analytics, taxonomy, search, logger, metrics.New()).Router()}
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
