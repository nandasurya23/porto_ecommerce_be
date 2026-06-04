package main

import (
	"log/slog"
	"os"

	"footwear-backend/internal/app"
	"footwear-backend/internal/config"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	server, err := app.New(cfg, logger)
	if err != nil {
		logger.Error("failed to bootstrap app", "error", err)
		os.Exit(1)
	}

	if err := server.Run(); err != nil {
		logger.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}
