package modules

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"footwear-backend/internal/config"
	"footwear-backend/internal/middleware"
	"footwear-backend/internal/shared/pagination"
	"footwear-backend/internal/shared/response"
	"footwear-backend/internal/shared/validator"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
)

type ordersModule struct {
	pool      *pgxpool.Pool
	cfg       config.Config
	logger    *slog.Logger
	jwtSecret []byte
}

func RegisterOrderRoutes(r *gin.RouterGroup, pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) {
	m := &ordersModule{pool: pool, cfg: cfg, logger: logger, jwtSecret: []byte(cfg.JWTSecret)}
	protected := r.Group("")
	protected.Use(middleware.Auth(m.jwtSecret))
	protected.POST("/orders", m.createOrder)
	protected.GET("/orders", m.listOrders)
	protected.GET("/orders/:id", m.orderDetail)

	admin := r.Group("/admin")
	admin.Use(middleware.Auth(m.jwtSecret), middleware.RequireRole("ADMIN", "SUPER_ADMIN"))
	admin.GET("/orders", m.adminOrders)
	admin.PATCH("/orders/:id/status", m.updateOrderStatus)
}

type createOrderRequest struct {
	AddressID     *string `json:"address_id"`
	PaymentMethod string  `json:"payment_method" binding:"required"`
}

func (m *ordersModule) createOrder(c *gin.Context) {
	user := currentUser(c)
	var req createOrderRequest
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

	var cartID uuid.UUID
	if err := tx.QueryRow(c.Request.Context(), `SELECT id FROM carts WHERE user_id=$1`, user.ID).Scan(&cartID); err != nil {
		if err == pgx.ErrNoRows {
			response.Error(c, http.StatusUnprocessableEntity, "Cart is empty", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}

	rows, err := tx.Query(c.Request.Context(), `
		SELECT ci.id, ci.product_variant_id, ci.quantity, pv.price, pv.stock, pv.status, pv.sku, pv.size, pv.color, p.id, p.name, p.slug
		FROM cart_items ci
		JOIN product_variants pv ON pv.id = ci.product_variant_id
		JOIN products p ON p.id = pv.product_id
		WHERE ci.cart_id=$1
		ORDER BY ci.created_at ASC
	`, cartID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer rows.Close()

	type cartLine struct {
		variantID   uuid.UUID
		quantity    int
		price       float64
		stock       int
		sku         string
		size        string
		color       string
		productID   uuid.UUID
		product     string
		productSlug string
	}
	lines := make([]cartLine, 0)
	var subtotal float64
	for rows.Next() {
		var line cartLine
		var variantStatus string
		var cartItemID uuid.UUID
		if err := rows.Scan(&cartItemID, &line.variantID, &line.quantity, &line.price, &line.stock, &variantStatus, &line.sku, &line.size, &line.color, &line.productID, &line.product, &line.productSlug); err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		if variantStatus != "ACTIVE" && variantStatus != "OUT_OF_STOCK" {
			response.Error(c, http.StatusUnprocessableEntity, "Variant is inactive", nil)
			return
		}
		if line.quantity > line.stock {
			response.Error(c, http.StatusUnprocessableEntity, "Insufficient stock", nil)
			return
		}
		subtotal += line.price * float64(line.quantity)
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		response.Error(c, http.StatusUnprocessableEntity, "Cart is empty", nil)
		return
	}

	if req.AddressID != nil && strings.TrimSpace(*req.AddressID) != "" {
		addressID, err := uuid.Parse(strings.TrimSpace(*req.AddressID))
		if err != nil {
			response.Error(c, http.StatusBadRequest, "Invalid address id", nil)
			return
		}
		var owner uuid.UUID
		if err := tx.QueryRow(c.Request.Context(), `SELECT user_id FROM addresses WHERE id=$1`, addressID).Scan(&owner); err != nil {
			response.Error(c, http.StatusNotFound, "Address not found", nil)
			return
		}
		if owner.String() != user.ID {
			response.Error(c, http.StatusForbidden, "You do not have permission", nil)
			return
		}
	}
	var addressParam any
	if req.AddressID != nil && strings.TrimSpace(*req.AddressID) != "" {
		addressID, _ := uuid.Parse(strings.TrimSpace(*req.AddressID))
		addressParam = addressID
	}

	shipping := computeShippingCost(subtotal)
	total := subtotal + shipping
	orderNumber := generateOrderNumber()
	var orderID uuid.UUID
	if err := tx.QueryRow(c.Request.Context(), `
		INSERT INTO orders (user_id, address_id, order_number, subtotal, shipping_cost, discount_amount, total_amount, payment_status, order_status)
		VALUES ($1,$2,$3,$4,$5,0,$6,'PENDING','PENDING_PAYMENT')
		RETURNING id
	`, user.ID, addressParam, orderNumber, subtotal, shipping, total).Scan(&orderID); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}

	for _, line := range lines {
		_, err := tx.Exec(c.Request.Context(), `
			INSERT INTO order_items (
				order_id, product_variant_id, product_name_snapshot, product_slug_snapshot, sku_snapshot, size_snapshot, color_snapshot, price_snapshot, quantity, subtotal
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`, orderID, line.variantID, line.product, line.productSlug, line.sku, line.size, line.color, line.price, line.quantity, line.price*float64(line.quantity))
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
	}

	paymentCode := generatePaymentCode()
	if _, err := tx.Exec(c.Request.Context(), `
		INSERT INTO payments (order_id, payment_code, method, amount, status)
		VALUES ($1,$2,$3,$4,'PENDING')
	`, orderID, paymentCode, normalizePaymentMethod(req.PaymentMethod), total); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}

	if _, err := tx.Exec(c.Request.Context(), `DELETE FROM cart_items WHERE cart_id=$1`, cartID); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if _, err := tx.Exec(c.Request.Context(), `
		INSERT INTO order_status_logs (order_id, old_status, new_status, note, changed_by)
		VALUES ($1, NULL, 'PENDING_PAYMENT', 'SYSTEM', NULL)
	`, orderID); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.Success(c, http.StatusCreated, "Order created successfully", gin.H{
		"order_id": orderID.String(), "order_number": orderNumber, "subtotal": subtotal, "shipping_cost": shipping, "total_amount": total,
		"payment_code": paymentCode, "payment_status": "PENDING", "order_status": "PENDING_PAYMENT",
	})
}

func (m *ordersModule) listOrders(c *gin.Context) {
	user := currentUser(c)
	page, limit := pagination.Parse(c.Query("page"), c.Query("limit"), 12)
	offset := (page - 1) * limit
	role := strings.ToUpper(user.Role)
	query := `SELECT id, user_id, order_number, subtotal, shipping_cost, discount_amount, total_amount, payment_status, order_status, created_at, updated_at FROM orders`
	args := []any{}
	if role == "CUSTOMER" {
		query += ` WHERE user_id=$1`
		args = append(args, user.ID)
	}
	query += ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)
	args = append(args, limit, offset)
	rows, err := m.pool.Query(c.Request.Context(), query, args...)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, owner uuid.UUID
		var orderNumber, paymentStatus, orderStatus string
		var subtotal, shippingCost, discount, total float64
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &owner, &orderNumber, &subtotal, &shippingCost, &discount, &total, &paymentStatus, &orderStatus, &createdAt, &updatedAt); err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		items = append(items, gin.H{"id": id.String(), "user_id": owner.String(), "order_number": orderNumber, "subtotal": subtotal, "shipping_cost": shippingCost, "discount_amount": discount, "total_amount": total, "payment_status": paymentStatus, "order_status": orderStatus, "created_at": createdAt, "updated_at": updatedAt})
	}
	var total int
	countQuery := `SELECT COUNT(*) FROM orders`
	countArgs := []any{}
	if role == "CUSTOMER" {
		countQuery += ` WHERE user_id=$1`
		countArgs = append(countArgs, user.ID)
	}
	if err := m.pool.QueryRow(c.Request.Context(), countQuery, countArgs...).Scan(&total); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.SuccessWithMeta(c, http.StatusOK, "Orders retrieved successfully", items, pagination.Meta{Page: page, Limit: limit, Total: total, TotalPages: totalPages(total, limit)})
}

func (m *ordersModule) adminOrders(c *gin.Context) {
	c.Set(middleware.ContextRole, "ADMIN")
	m.listOrders(c)
}

func (m *ordersModule) orderDetail(c *gin.Context) {
	user := currentUser(c)
	id, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	allowed := strings.ToUpper(user.Role) != "CUSTOMER"
	var owner string
	var orderNumber, paymentStatus, orderStatus string
	var subtotal, shipping, discount, total float64
	var addressID *string
	var createdAt, updatedAt time.Time
	if err := m.pool.QueryRow(c.Request.Context(), `
		SELECT user_id, order_number, subtotal, shipping_cost, discount_amount, total_amount, payment_status, order_status, COALESCE(address_id::text, ''), created_at, updated_at
		FROM orders WHERE id=$1
	`, id).Scan(&owner, &orderNumber, &subtotal, &shipping, &discount, &total, &paymentStatus, &orderStatus, &addressID, &createdAt, &updatedAt); err != nil {
		if err == pgx.ErrNoRows {
			response.Error(c, http.StatusNotFound, "Order not found", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if !allowed && owner != user.ID {
		response.Error(c, http.StatusForbidden, "You do not have permission", nil)
		return
	}
	items, _ := m.orderItems(c.Request.Context(), id)
	response.Success(c, http.StatusOK, "Order retrieved successfully", gin.H{
		"id": id.String(), "user_id": owner, "address_id": addressID, "order_number": orderNumber, "subtotal": subtotal, "shipping_cost": shipping,
		"discount_amount": discount, "total_amount": total, "payment_status": paymentStatus, "order_status": orderStatus, "items": items, "created_at": createdAt, "updated_at": updatedAt,
	})
}

func (m *ordersModule) orderShipment(c *gin.Context) {
	user := currentUser(c)
	id, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	var owner string
	if err := m.pool.QueryRow(c.Request.Context(), `SELECT user_id FROM orders WHERE id=$1`, id).Scan(&owner); err != nil {
		response.Error(c, http.StatusNotFound, "Order not found", nil)
		return
	}
	if strings.ToUpper(user.Role) == "CUSTOMER" && owner != user.ID {
		response.Error(c, http.StatusForbidden, "You do not have permission", nil)
		return
	}
	var shipmentID uuid.UUID
	var courier, status string
	var serviceName, trackingNumber *string
	var shippedAt, deliveredAt, createdAt, updatedAt *time.Time
	if err := m.pool.QueryRow(c.Request.Context(), `SELECT id, courier, service_name, tracking_number, status, shipped_at, delivered_at, created_at, updated_at FROM shipments WHERE order_id=$1`, id).
		Scan(&shipmentID, &courier, &serviceName, &trackingNumber, &status, &shippedAt, &deliveredAt, &createdAt, &updatedAt); err != nil {
		response.Error(c, http.StatusNotFound, "Shipment not found", nil)
		return
	}
	response.Success(c, http.StatusOK, "Shipment retrieved successfully", gin.H{
		"id": shipmentID.String(), "courier": courier, "service_name": serviceName, "tracking_number": trackingNumber, "status": status,
		"shipped_at": shippedAt, "delivered_at": deliveredAt, "created_at": createdAt, "updated_at": updatedAt,
	})
}

func (m *ordersModule) updateOrderStatus(c *gin.Context) {
	id, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
		Note   string `json:"note"`
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
	var oldStatus string
	if err := tx.QueryRow(c.Request.Context(), `SELECT order_status FROM orders WHERE id=$1 FOR UPDATE`, id).Scan(&oldStatus); err != nil {
		response.Error(c, http.StatusNotFound, "Order not found", nil)
		return
	}
	newStatus := strings.ToUpper(strings.TrimSpace(req.Status))
	if _, err := tx.Exec(c.Request.Context(), `UPDATE orders SET order_status=$1, updated_at=NOW() WHERE id=$2`, newStatus, id); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if _, err := tx.Exec(c.Request.Context(), `
		INSERT INTO order_status_logs (order_id, old_status, new_status, note, changed_by)
		VALUES ($1,$2,$3,$4,$5)
	`, id, oldStatus, newStatus, req.Note, currentUser(c).ID); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.Success(c, http.StatusOK, "Order status updated successfully", gin.H{})
}

func (m *ordersModule) orderItems(ctx context.Context, orderID uuid.UUID) ([]gin.H, error) {
	rows, err := m.pool.Query(ctx, `SELECT id, COALESCE(product_variant_id::text, ''), product_name_snapshot, product_slug_snapshot, sku_snapshot, size_snapshot, color_snapshot, price_snapshot, quantity, subtotal, created_at FROM order_items WHERE order_id=$1 ORDER BY created_at ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id uuid.UUID
		var variantID string
		var productName, sku, size, color string
		var productSlug *string
		var price float64
		var quantity int
		var subtotal float64
		var createdAt time.Time
		if err := rows.Scan(&id, &variantID, &productName, &productSlug, &sku, &size, &color, &price, &quantity, &subtotal, &createdAt); err != nil {
			return nil, err
		}
		items = append(items, gin.H{"id": id.String(), "product_variant_id": blankOrPointer(variantID), "product_name_snapshot": productName, "product_slug_snapshot": productSlug, "sku_snapshot": sku, "size_snapshot": size, "color_snapshot": color, "price_snapshot": price, "quantity": quantity, "subtotal": subtotal, "created_at": createdAt})
	}
	return items, nil
}

func computeShippingCost(subtotal float64) float64 {
	if subtotal >= 500000 {
		return 0
	}
	return 20000
}

func generateOrderNumber() string {
	return fmt.Sprintf("ORD-%s-%d", time.Now().UTC().Format("20060102"), time.Now().UTC().UnixNano()%1000000)
}

func generatePaymentCode() string {
	return fmt.Sprintf("PAY-%s-%d", time.Now().UTC().Format("20060102"), time.Now().UTC().UnixNano()%1000000)
}

func normalizePaymentMethod(v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	if v == "" {
		return "BANK_TRANSFER"
	}
	switch v {
	case "BANK_TRANSFER", "VIRTUAL_ACCOUNT", "QRIS_SIMULATION", "COD":
		return v
	default:
		return "BANK_TRANSFER"
	}
}
