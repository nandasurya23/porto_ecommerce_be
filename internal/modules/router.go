package modules

import (
	"log/slog"

	"footwear-backend/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterRoutes(r *gin.RouterGroup, pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) {
	// implemented in module files
	RegisterAuthRoutes(r, pool, cfg, logger)
	RegisterAddressRoutes(r, pool, cfg, logger)
	RegisterCatalogRoutes(r, pool, cfg, logger)
	RegisterCartRoutes(r, pool, cfg, logger)
	RegisterOrderRoutes(r, pool, cfg, logger)
	RegisterPaymentRoutes(r, pool, cfg, logger)
	RegisterShipmentRoutes(r, pool, cfg, logger)
	RegisterDashboardRoutes(r, pool, cfg, logger)
}
