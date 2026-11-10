package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tishanyq/fpl-edge/backend/internal/auth"
	"github.com/tishanyq/fpl-edge/backend/internal/config"
	"github.com/tishanyq/fpl-edge/backend/internal/football"
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
	} else {
		syncFootballData(ctx, logger, dataStore)
		syncLiveData(ctx, logger, dataStore)
		go refreshFootballData(ctx, logger, dataStore)
		go refreshLiveData(ctx, logger, dataStore)
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

func refreshLiveData(ctx context.Context, logger *slog.Logger, dataStore *store.Store) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			syncLiveData(ctx, logger, dataStore)
		}
	}
}

func syncLiveData(ctx context.Context, logger *slog.Logger, dataStore *store.Store) {
	locked, release, err := dataStore.TryLiveSyncLease(ctx)
	if err != nil {
		logger.Warn("live sync lease failed", "error", err)
		return
	}
	if !locked {
		return
	}
	defer release()
	gameweek, needed, err := dataStore.LiveSyncTarget(ctx)
	if err != nil || !needed {
		if err != nil {
			logger.Warn("live sync target failed", "error", err)
		}
		return
	}
	data, err := football.NewClient().FetchLive(ctx, gameweek)
	if err != nil {
		logger.Warn("live gameweek sync skipped", "gameweek", gameweek, "error", err)
		return
	}
	if err = dataStore.SyncLiveData(ctx, data); err != nil {
		logger.Warn("live gameweek sync failed", "gameweek", gameweek, "error", err)
		return
	}
	logger.Info("live gameweek synchronized", "gameweek", gameweek, "players", len(data.Players), "fixtures", len(data.Fixtures))
}

func refreshFootballData(ctx context.Context, logger *slog.Logger, dataStore *store.Store) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			syncFootballData(ctx, logger, dataStore)
		}
	}
}

func syncFootballData(ctx context.Context, logger *slog.Logger, dataStore *store.Store) {
	footballData, err := football.NewClient().Fetch(ctx)
	if err != nil {
		logger.Warn("football data sync skipped", "error", err)
		return
	}
	if err = dataStore.SyncFootballData(ctx, footballData); err != nil {
		logger.Warn("football data sync failed", "error", err)
		return
	}
	logger.Info("football data synchronized", "season", footballData.SeasonName, "players", len(footballData.Players))
}
