package modules

import (
	"net/http"
	"time"

	"footwear-backend/internal/config"
	"footwear-backend/internal/middleware"
	"footwear-backend/internal/shared/pagination"
	"footwear-backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
)

type dashboardModule struct {
	pool      *pgxpool.Pool
	cfg       config.Config
	logger    *slog.Logger
	jwtSecret []byte
}

func RegisterDashboardRoutes(r *gin.RouterGroup, pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) {
	m := &dashboardModule{pool: pool, cfg: cfg, logger: logger, jwtSecret: []byte(cfg.JWTSecret)}
	admin := r.Group("/admin")
	admin.Use(middleware.Auth(m.jwtSecret), middleware.RequireRole("ADMIN", "SUPER_ADMIN", "WAREHOUSE"))
	admin.GET("/dashboard/summary", m.summary)
	admin.GET("/dashboard/sales", m.sales)
	admin.GET("/dashboard/orders-by-status", m.ordersByStatus)
	admin.GET("/dashboard/low-stock", m.lowStock)
}

func (m *dashboardModule) summary(c *gin.Context) {
	var totalRevenue float64
	var totalOrders, pending, processing, delivered, lowStock int
	_ = m.pool.QueryRow(c.Request.Context(), `SELECT COALESCE(SUM(total_amount),0) FROM orders WHERE payment_status='PAID'`).Scan(&totalRevenue)
	_ = m.pool.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM orders`).Scan(&totalOrders)
	_ = m.pool.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM orders WHERE order_status='PENDING_PAYMENT'`).Scan(&pending)
	_ = m.pool.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM orders WHERE order_status='PROCESSING'`).Scan(&processing)
	_ = m.pool.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM orders WHERE order_status='DELIVERED'`).Scan(&delivered)
	_ = m.pool.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM product_variants WHERE stock <= 5`).Scan(&lowStock)
	response.Success(c, http.StatusOK, "Dashboard summary retrieved successfully", gin.H{
		"total_revenue": totalRevenue, "total_orders": totalOrders, "pending_payment_orders": pending,
		"processing_orders": processing, "delivered_orders": delivered, "low_stock_variants": lowStock,
	})
}

func (m *dashboardModule) sales(c *gin.Context) {
	page, limit := pagination.Parse(c.Query("page"), c.Query("limit"), 12)
	_ = page
	_ = limit
	rows, err := m.pool.Query(c.Request.Context(), `
		SELECT DATE(created_at), COALESCE(SUM(total_amount),0)
		FROM orders WHERE payment_status='PAID'
		GROUP BY DATE(created_at)
		ORDER BY DATE(created_at) DESC
		LIMIT 30
	`)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var date time.Time
		var total float64
		if err := rows.Scan(&date, &total); err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		items = append(items, gin.H{"date": date, "total_revenue": total})
	}
	response.Success(c, http.StatusOK, "Sales retrieved successfully", items)
}

func (m *dashboardModule) ordersByStatus(c *gin.Context) {
	rows, err := m.pool.Query(c.Request.Context(), `SELECT order_status, COUNT(*) FROM orders GROUP BY order_status ORDER BY order_status`)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var status string
		var total int
		if err := rows.Scan(&status, &total); err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		items = append(items, gin.H{"order_status": status, "total": total})
	}
	response.Success(c, http.StatusOK, "Orders by status retrieved successfully", items)
}

func (m *dashboardModule) lowStock(c *gin.Context) {
	rows, err := m.pool.Query(c.Request.Context(), `
		SELECT pv.id, p.name, pv.sku, pv.size, pv.color, pv.stock, pv.price
		FROM product_variants pv
		JOIN products p ON p.id = pv.product_id
		WHERE pv.stock <= 5
		ORDER BY pv.stock ASC, p.name ASC
		LIMIT 50
	`)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, name, sku, size, color string
		var stock int
		var price float64
		if err := rows.Scan(&id, &name, &sku, &size, &color, &stock, &price); err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		items = append(items, gin.H{"id": id, "product_name": name, "sku": sku, "size": size, "color": color, "stock": stock, "price": price})
	}
	response.Success(c, http.StatusOK, "Low stock variants retrieved successfully", items)
}
