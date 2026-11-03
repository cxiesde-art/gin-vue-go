package config

import "os"

type Config struct {
	Address     string
	DatabaseURL string
	JWTSecret   string
	AdminEmail  string
	AdminPass   string
}

func FromEnv() Config {
	return Config{
		Address:     env("BLOG_ADDRESS", ":8080"),
		DatabaseURL: env("BLOG_DATABASE_URL", "blog.db"),
		JWTSecret:   env("BLOG_JWT_SECRET", "development-secret-change-me"),
		AdminEmail:  env("BLOG_ADMIN_EMAIL", "admin@example.com"),
		AdminPass:   env("BLOG_ADMIN_PASSWORD", "admin123"),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
