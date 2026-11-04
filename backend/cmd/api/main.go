package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tishanyq/fpl-edge/backend/internal/auth"
	"github.com/tishanyq/fpl-edge/backend/internal/config"
	"github.com/tishanyq/fpl-edge/backend/internal/httpapi"
	"github.com/tishanyq/fpl-edge/backend/internal/store"
	"github.com/tishanyq/fpl-edge/backend/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		logger.Error("invalid database URL", "error", err)
		os.Exit(1)
	}
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 1
	poolConfig.MaxConnLifetime = time.Hour
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err = migrations.Up(ctx, pool); err != nil {
		logger.Error("apply migrations", "error", err)
		os.Exit(1)
	}
	dataStore := store.New(pool)
	if cfg.Environment == "development" {
		if err = dataStore.SeedDevelopmentData(ctx); err != nil {
			logger.Error("seed development football data", "error", err)
			os.Exit(1)
		}
	}
	authService := auth.New(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	e := httpapi.New(cfg, dataStore, authService, logger)
	go func() {
		logger.Info("API listening", "address", cfg.HTTPAddr, "environment", cfg.Environment)
		if err := e.Start(cfg.HTTPAddr); err != nil && err != http.ErrServerClosed {
			logger.Error("API server failed", "error", err)
			cancel()
		}
	}()
	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
