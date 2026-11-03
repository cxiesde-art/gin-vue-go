package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/cxiesde-art/gin-vue-blog-go/internal/auth"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/config"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/handler"
	"github.com/cxiesde-art/gin-vue-blog-go/internal/repository"
)

func main() {
	cfg := config.FromEnv()
	repo, err := repository.Open(cfg.DatabaseURL)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	if err := repo.SeedAdmin(context.Background(), cfg.AdminEmail, auth.HashPassword(cfg.AdminPass)); err != nil {
		slog.Error("seed administrator", "error", err)
		os.Exit(1)
	}

	server := &http.Server{Addr: cfg.Address, Handler: handler.New(cfg, repo).Router()}
	slog.Info("blog API listening", "address", cfg.Address, "database", cfg.DatabaseURL)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("serve HTTP", "error", err)
		os.Exit(1)
	}
}
