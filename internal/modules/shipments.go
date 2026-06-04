package modules

import (
	"net/http"
	"strings"
	"time"

	"footwear-backend/internal/config"
	"footwear-backend/internal/middleware"
	"footwear-backend/internal/shared/response"
	"footwear-backend/internal/shared/validator"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
)

type shipmentsModule struct {
	pool      *pgxpool.Pool
	cfg       config.Config
	logger    *slog.Logger
	jwtSecret []byte
}

func RegisterShipmentRoutes(r *gin.RouterGroup, pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) {
	m := &shipmentsModule{pool: pool, cfg: cfg, logger: logger, jwtSecret: []byte(cfg.JWTSecret)}
	protected := r.Group("")
	protected.Use(middleware.Auth(m.jwtSecret))
	protected.GET("/orders/:id/shipment", m.orderShipment)

	admin := r.Group("/admin")
	admin.Use(middleware.Auth(m.jwtSecret), middleware.RequireRole("ADMIN", "SUPER_ADMIN", "WAREHOUSE"))
	admin.POST("/orders/:id/shipment", m.createShipment)
	admin.PATCH("/shipments/:id/status", m.updateShipmentStatus)
}

type shipmentCreateRequest struct {
	Courier        string `json:"courier" binding:"required"`
	ServiceName    string `json:"service_name"`
	TrackingNumber string `json:"tracking_number"`
}

func (m *shipmentsModule) createShipment(c *gin.Context) {
	orderID, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	var req shipmentCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "Validation error", validator.NormalizeErrors(err))
		return
	}
	tx, err := m.pool.Begin(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer func() { _ = tx.Rollback(c.Request.Context()) }()
	var orderStatus, paymentStatus string
	if err := tx.QueryRow(c.Request.Context(), `SELECT order_status, payment_status FROM orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&orderStatus, &paymentStatus); err != nil {
		if err == pgx.ErrNoRows {
			response.Error(c, http.StatusNotFound, "Order not found", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if paymentStatus != "PAID" || (orderStatus != "PROCESSING" && orderStatus != "PACKED") {
		response.Error(c, http.StatusUnprocessableEntity, "Shipment cannot be created for unpaid order", nil)
		return
	}
	var id uuid.UUID
	if err := tx.QueryRow(c.Request.Context(), `
		INSERT INTO shipments (order_id, courier, service_name, tracking_number, status)
		VALUES ($1,$2,$3,$4,'WAITING_FOR_PICKUP')
		RETURNING id
	`, orderID, req.Courier, nullString(req.ServiceName), nullString(req.TrackingNumber)).Scan(&id); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.Success(c, http.StatusCreated, "Shipment created successfully", gin.H{"id": id.String()})
}

func (m *shipmentsModule) updateShipmentStatus(c *gin.Context) {
	id, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Status         string `json:"status" binding:"required"`
		TrackingNumber string `json:"tracking_number"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "Validation error", validator.NormalizeErrors(err))
		return
	}
	tx, err := m.pool.Begin(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer func() { _ = tx.Rollback(c.Request.Context()) }()
	var orderID uuid.UUID
	var oldStatus string
	if err := tx.QueryRow(c.Request.Context(), `SELECT order_id, status FROM shipments WHERE id=$1 FOR UPDATE`, id).Scan(&orderID, &oldStatus); err != nil {
		if err == pgx.ErrNoRows {
			response.Error(c, http.StatusNotFound, "Shipment not found", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	newStatus := strings.ToUpper(strings.TrimSpace(req.Status))
	if (newStatus == "PICKED_UP" || newStatus == "IN_TRANSIT" || newStatus == "OUT_FOR_DELIVERY" || newStatus == "DELIVERED") && strings.TrimSpace(req.TrackingNumber) == "" {
		response.Error(c, http.StatusUnprocessableEntity, "Tracking number required before shipped", nil)
		return
	}
	if _, err := tx.Exec(c.Request.Context(), `
		UPDATE shipments
		SET status=$1::shipment_status, tracking_number=COALESCE(NULLIF($2,''), tracking_number),
		    shipped_at=CASE WHEN $1='PICKED_UP' OR $1='IN_TRANSIT' OR $1='OUT_FOR_DELIVERY' OR $1='DELIVERED' THEN COALESCE(shipped_at, NOW()) ELSE shipped_at END,
		    delivered_at=CASE WHEN $1='DELIVERED' THEN NOW() ELSE delivered_at END,
		    updated_at=NOW()
		WHERE id=$3
	`, newStatus, req.TrackingNumber, id); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if newStatus == "DELIVERED" {
		if _, err := tx.Exec(c.Request.Context(), `UPDATE orders SET order_status='DELIVERED', updated_at=NOW() WHERE id=$1`, orderID); err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		if _, err := tx.Exec(c.Request.Context(), `
			INSERT INTO order_status_logs (order_id, old_status, new_status, note, changed_by)
			VALUES ($1,$2,'DELIVERED','SYSTEM',NULL)
		`, orderID, "SHIPPED"); err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.Success(c, http.StatusOK, "Shipment status updated successfully", gin.H{})
}

func (m *shipmentsModule) orderShipment(c *gin.Context) {
	user := currentUser(c)
	orderID, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	var owner string
	if err := m.pool.QueryRow(c.Request.Context(), `SELECT user_id FROM orders WHERE id=$1`, orderID).Scan(&owner); err != nil {
		response.Error(c, http.StatusNotFound, "Order not found", nil)
		return
	}
	if strings.ToUpper(user.Role) == "CUSTOMER" && owner != user.ID {
		response.Error(c, http.StatusForbidden, "You do not have permission", nil)
		return
	}
	var id uuid.UUID
	var courier, status string
	var serviceName, trackingNumber *string
	var shippedAt, deliveredAt, createdAt, updatedAt *time.Time
	if err := m.pool.QueryRow(c.Request.Context(), `
		SELECT id, courier, service_name, tracking_number, status, shipped_at, delivered_at, created_at, updated_at
		FROM shipments WHERE order_id=$1
	`, orderID).Scan(&id, &courier, &serviceName, &trackingNumber, &status, &shippedAt, &deliveredAt, &createdAt, &updatedAt); err != nil {
		if err == pgx.ErrNoRows {
			response.Error(c, http.StatusNotFound, "Shipment not found", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.Success(c, http.StatusOK, "Shipment retrieved successfully", gin.H{
		"id": id.String(), "courier": courier, "service_name": serviceName, "tracking_number": trackingNumber, "status": status,
		"shipped_at": shippedAt, "delivered_at": deliveredAt, "created_at": createdAt, "updated_at": updatedAt,
	})
}
