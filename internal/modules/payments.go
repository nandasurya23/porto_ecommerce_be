package modules

import (
	"context"
	"net/http"
	"strings"
	"time"

	"footwear-backend/internal/config"
	"footwear-backend/internal/middleware"
	"footwear-backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
)

type paymentsModule struct {
	pool      *pgxpool.Pool
	cfg       config.Config
	logger    *slog.Logger
	jwtSecret []byte
}

func RegisterPaymentRoutes(r *gin.RouterGroup, pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) {
	m := &paymentsModule{pool: pool, cfg: cfg, logger: logger, jwtSecret: []byte(cfg.JWTSecret)}
	group := r.Group("/payments")
	group.Use(middleware.Auth(m.jwtSecret))
	group.POST("/:order_id/simulate-success", m.simulateSuccess)
	group.POST("/:order_id/simulate-failed", m.simulateFailed)
	group.POST("/:order_id/expire", m.expire)

	admin := r.Group("/admin")
	admin.Use(middleware.Auth(m.jwtSecret), middleware.RequireRole("ADMIN", "SUPER_ADMIN"))
	admin.GET("/payments", m.adminPayments)
}

func (m *paymentsModule) simulateSuccess(c *gin.Context) {
	user := currentUser(c)
	m.processPayment(c, user.ID, true)
}

func (m *paymentsModule) simulateFailed(c *gin.Context) {
	user := currentUser(c)
	m.processPayment(c, user.ID, false)
}

func (m *paymentsModule) expire(c *gin.Context) {
	user := currentUser(c)
	orderID, ok := mustUUIDParam(c, "order_id")
	if !ok {
		return
	}
	tx, err := m.pool.Begin(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer func() { _ = tx.Rollback(c.Request.Context()) }()
	var owner string
	var paymentStatus, orderStatus string
	var paymentID uuid.UUID
	if err := tx.QueryRow(c.Request.Context(), `
		SELECT o.user_id, p.id, p.status, o.order_status
		FROM orders o
		JOIN payments p ON p.order_id=o.id
		WHERE o.id=$1 FOR UPDATE
	`, orderID).Scan(&owner, &paymentID, &paymentStatus, &orderStatus); err != nil {
		if err == pgx.ErrNoRows {
			response.Error(c, http.StatusNotFound, "Order not found", nil)
			return
		}
		m.logger.Error("payment expire failed", "step", "load-order-payment", "error", err)
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if strings.ToUpper(user.Role) == "CUSTOMER" && owner != user.ID {
		response.Error(c, http.StatusForbidden, "You do not have permission", nil)
		return
	}
	if paymentStatus != "PENDING" {
		response.Success(c, http.StatusOK, "Payment already finalized", gin.H{"payment_status": paymentStatus, "order_status": orderStatus})
		return
	}
	if _, err := tx.Exec(c.Request.Context(), `UPDATE payments SET status='EXPIRED', expired_at=NOW(), updated_at=NOW() WHERE id=$1`, paymentID); err != nil {
		m.logger.Error("payment expire failed", "step", "update-payment", "error", err)
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if _, err := tx.Exec(c.Request.Context(), `UPDATE orders SET payment_status='EXPIRED', order_status='CANCELLED', updated_at=NOW() WHERE id=$1`, orderID); err != nil {
		m.logger.Error("payment expire failed", "step", "update-order", "error", err)
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if _, err := tx.Exec(c.Request.Context(), `INSERT INTO order_status_logs (order_id, old_status, new_status, note, changed_by) VALUES ($1,$2,'CANCELLED','SYSTEM',NULL)`, orderID, orderStatus); err != nil {
		m.logger.Error("payment expire failed", "step", "insert-order-status-log", "error", err)
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		m.logger.Error("payment expire failed", "step", "commit", "error", err)
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.Success(c, http.StatusOK, "Payment expired successfully", gin.H{})
}

func (m *paymentsModule) processPayment(c *gin.Context, requesterID string, success bool) {
	orderID, ok := mustUUIDParam(c, "order_id")
	if !ok {
		return
	}
	tx, err := m.pool.Begin(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer func() { _ = tx.Rollback(c.Request.Context()) }()

	var owner string
	var paymentID uuid.UUID
	var paymentStatus, orderStatus string
	var subtotal, shipping, total float64
	if err := tx.QueryRow(c.Request.Context(), `
		SELECT o.user_id, p.id, p.status, o.order_status, o.subtotal, o.shipping_cost, o.total_amount
		FROM orders o
		JOIN payments p ON p.order_id=o.id
		WHERE o.id=$1 FOR UPDATE
	`, orderID).Scan(&owner, &paymentID, &paymentStatus, &orderStatus, &subtotal, &shipping, &total); err != nil {
		if err == pgx.ErrNoRows {
			response.Error(c, http.StatusNotFound, "Order not found", nil)
			return
		}
		m.logger.Error("payment success failed", "step", "load-order-payment", "error", err)
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if owner != requesterID && strings.ToUpper(currentUser(c).Role) == "CUSTOMER" {
		response.Error(c, http.StatusForbidden, "You do not have permission", nil)
		return
	}
	if paymentStatus != "PENDING" || orderStatus != "PENDING_PAYMENT" {
		response.Success(c, http.StatusOK, "Payment already finalized", gin.H{"payment_status": paymentStatus, "order_status": orderStatus})
		return
	}
	if !success {
		if _, err := tx.Exec(c.Request.Context(), `UPDATE payments SET status='FAILED', updated_at=NOW() WHERE id=$1`, paymentID); err != nil {
			m.logger.Error("payment failed flow", "step", "update-payment-failed", "error", err)
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		if _, err := tx.Exec(c.Request.Context(), `UPDATE orders SET payment_status='FAILED', order_status='CANCELLED', updated_at=NOW() WHERE id=$1`, orderID); err != nil {
			m.logger.Error("payment failed flow", "step", "update-order-failed", "error", err)
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		if _, err := tx.Exec(c.Request.Context(), `INSERT INTO order_status_logs (order_id, old_status, new_status, note, changed_by) VALUES ($1,$2,'CANCELLED','SYSTEM',NULL)`, orderID, orderStatus); err != nil {
			m.logger.Error("payment failed flow", "step", "insert-order-status-log", "error", err)
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		if err := tx.Commit(c.Request.Context()); err != nil {
			m.logger.Error("payment failed flow", "step", "commit", "error", err)
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		response.Success(c, http.StatusOK, "Payment marked as failed", gin.H{})
		return
	}

	items, err := m.paymentItems(c.Request.Context(), tx, orderID)
	if err != nil {
		m.logger.Error("payment success failed", "step", "load-items", "error", err)
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	for _, item := range items {
		var stock int
		if err := tx.QueryRow(c.Request.Context(), `SELECT stock FROM product_variants WHERE id=$1 FOR UPDATE`, item.variantID).Scan(&stock); err != nil {
			m.logger.Error("payment success failed", "step", "lock-variant-stock", "variant_id", item.variantID.String(), "error", err)
			response.Error(c, http.StatusNotFound, "Variant not found", nil)
			return
		}
		if item.quantity > stock {
			response.Error(c, http.StatusUnprocessableEntity, "Insufficient stock", nil)
			return
		}
	}
	if _, err := tx.Exec(c.Request.Context(), `UPDATE payments SET status='PAID', paid_at=NOW(), updated_at=NOW() WHERE id=$1`, paymentID); err != nil {
		m.logger.Error("payment success failed", "step", "update-payment-paid", "error", err)
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if _, err := tx.Exec(c.Request.Context(), `UPDATE orders SET payment_status='PAID', order_status='PROCESSING', updated_at=NOW() WHERE id=$1`, orderID); err != nil {
		m.logger.Error("payment success failed", "step", "update-order-processing", "error", err)
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	for _, item := range items {
		var stock int
		if err := tx.QueryRow(c.Request.Context(), `SELECT stock FROM product_variants WHERE id=$1 FOR UPDATE`, item.variantID).Scan(&stock); err != nil {
			m.logger.Error("payment success failed", "step", "reload-variant-stock", "variant_id", item.variantID.String(), "error", err)
			response.Error(c, http.StatusNotFound, "Variant not found", nil)
			return
		}
		next := stock - item.quantity
		if _, err := tx.Exec(c.Request.Context(), `UPDATE product_variants SET stock=$1, status=CASE WHEN $1=0 THEN 'OUT_OF_STOCK'::variant_status ELSE 'ACTIVE'::variant_status END, updated_at=NOW() WHERE id=$2`, next, item.variantID); err != nil {
			m.logger.Error("payment success failed", "step", "update-variant-stock", "variant_id", item.variantID.String(), "error", err)
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		_, _ = tx.Exec(c.Request.Context(), `
			INSERT INTO inventory_logs (product_variant_id, type, quantity, previous_stock, current_stock, note, created_by)
			VALUES ($1,'SALE',$2,$3,$4,$5,$6)
		`, item.variantID, -item.quantity, stock, next, "Payment success deduction", nil)
	}
	if _, err := tx.Exec(c.Request.Context(), `
		INSERT INTO order_status_logs (order_id, old_status, new_status, note, changed_by)
		VALUES ($1,$2,'PROCESSING','SYSTEM',NULL)
	`, orderID, orderStatus); err != nil {
		m.logger.Error("payment success failed", "step", "insert-order-status-log", "error", err)
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		m.logger.Error("payment success failed", "step", "commit", "error", err)
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.Success(c, http.StatusOK, "Payment success simulated", gin.H{"payment_status": "PAID", "order_status": "PROCESSING", "subtotal": subtotal, "shipping_cost": shipping, "total_amount": total})
}

type paymentLine struct {
	variantID uuid.UUID
	quantity  int
}

func (m *paymentsModule) paymentItems(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) ([]paymentLine, error) {
	rows, err := tx.Query(ctx, `SELECT product_variant_id, quantity FROM order_items WHERE order_id=$1`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	lines := make([]paymentLine, 0)
	for rows.Next() {
		var line paymentLine
		if err := rows.Scan(&line.variantID, &line.quantity); err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func (m *paymentsModule) adminPayments(c *gin.Context) {
	rows, err := m.pool.Query(c.Request.Context(), `
		SELECT p.id, p.payment_code, p.method, p.amount, p.status, p.paid_at, p.expired_at, p.created_at, p.updated_at, o.id, o.order_number
		FROM payments p
		JOIN orders o ON o.id = p.order_id
		ORDER BY p.created_at DESC
		LIMIT 100
	`)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, orderID uuid.UUID
		var paymentCode, method, status, orderNumber string
		var amount float64
		var paidAt, expiredAt, createdAt, updatedAt *time.Time
		if err := rows.Scan(&id, &paymentCode, &method, &amount, &status, &paidAt, &expiredAt, &createdAt, &updatedAt, &orderID, &orderNumber); err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		items = append(items, gin.H{"id": id.String(), "payment_code": paymentCode, "method": method, "amount": amount, "status": status, "paid_at": paidAt, "expired_at": expiredAt, "created_at": createdAt, "updated_at": updatedAt, "order_id": orderID.String(), "order_number": orderNumber})
	}
	response.Success(c, http.StatusOK, "Payments retrieved successfully", items)
}
