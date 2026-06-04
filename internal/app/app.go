package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"footwear-backend/internal/config"
	"footwear-backend/internal/database"
	"footwear-backend/internal/middleware"
	"footwear-backend/internal/modules"
	"github.com/gin-gonic/gin"
)

type App struct {
	cfg    config.Config
	logger *slog.Logger
	db     *database.DB
	router *gin.Engine
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db, err := database.Open(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}
	if err := ensureDirs(cfg); err != nil {
		db.Close()
		return nil, err
	}
	if err := db.Migrate(ctx, "migrations"); err != nil {
		db.Close()
		return nil, err
	}
	if cfg.ResetDemoData {
		if strings.EqualFold(cfg.AppEnv, "production") {
			db.Close()
			return nil, fmt.Errorf("RESET_DEMO_DATA is disabled in production")
		}
		if err := db.ResetDemoData(ctx); err != nil {
			db.Close()
			return nil, err
		}
	}
	if err := db.SeedDemoData(ctx, cfg, logger); err != nil {
		db.Close()
		return nil, err
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.BodyLimit(cfg.MaxBodySizeMB))
	router.Use(middleware.CORS(cfg.FrontendURL))
	router.Use(middleware.ErrorHandler())

	api := router.Group("/api/v1")
	modules.RegisterRoutes(api, db.Pool, cfg, logger)
	api.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "ok", "data": gin.H{"status": "healthy"}})
	})

	router.Static("/uploads", cfg.UploadDir)

	return &App{cfg: cfg, logger: logger, db: db, router: router}, nil
}

func (a *App) Run() error {
	defer a.db.Close()
	addr := fmt.Sprintf("0.0.0.0:%s", a.cfg.AppPort)
	server := &http.Server{
		Addr:              addr,
		Handler:           a.router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	a.logger.Info("server starting", "addr", addr)
	return server.ListenAndServe()
}

func ensureDirs(cfg config.Config) error {
	return os.MkdirAll(filepath.Join(cfg.UploadDir, "products"), 0o755)
}
