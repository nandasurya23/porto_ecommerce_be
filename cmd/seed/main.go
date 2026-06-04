package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"footwear-backend/internal/config"
	"footwear-backend/internal/database"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	if strings.EqualFold(cfg.AppEnv, "production") && cfg.ResetDemoData {
		logger.Error("reset demo data is disabled in production")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db, err := database.Open(ctx, cfg, logger)
	if err != nil {
		logger.Error("failed to connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Migrate(ctx, "migrations"); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	if cfg.ResetDemoData {
		if err := db.ResetDemoData(ctx); err != nil {
			logger.Error("failed to reset demo data", "error", err)
			os.Exit(1)
		}
	}

	if err := db.SeedDemoData(ctx, cfg, logger); err != nil {
		logger.Error("failed to seed demo data", "error", err)
		os.Exit(1)
	}

	logger.Info("demo data seeded successfully")
}
