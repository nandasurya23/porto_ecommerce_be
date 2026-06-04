package modules

import (
	"context"
	"net/http"
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

type cartModule struct {
	pool      *pgxpool.Pool
	cfg       config.Config
	logger    *slog.Logger
	jwtSecret []byte
}

func RegisterCartRoutes(r *gin.RouterGroup, pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) {
	m := &cartModule{pool: pool, cfg: cfg, logger: logger, jwtSecret: []byte(cfg.JWTSecret)}
	group := r.Group("/cart")
	group.Use(middleware.Auth(m.jwtSecret), middleware.RequireRole("CUSTOMER"))
	group.GET("", m.getCart)
	group.POST("/items", m.addItem)
	group.PATCH("/items/:id", m.updateItem)
	group.DELETE("/items/:id", m.deleteItem)
	group.DELETE("/items", m.clearCart)
}

type cartItemRequest struct {
	ProductVariantID string `json:"product_variant_id" binding:"required"`
	Quantity         int    `json:"quantity" binding:"required"`
}

func (m *cartModule) getOrCreateCart(ctx context.Context, userID string) (uuid.UUID, error) {
	var cartID uuid.UUID
	err := m.pool.QueryRow(ctx, `SELECT id FROM carts WHERE user_id=$1`, userID).Scan(&cartID)
	if err == nil {
		return cartID, nil
	}
	if err != pgx.ErrNoRows {
		return uuid.Nil, err
	}
	if err := m.pool.QueryRow(ctx, `INSERT INTO carts (user_id) VALUES ($1) RETURNING id`, userID).Scan(&cartID); err != nil {
		return uuid.Nil, err
	}
	return cartID, nil
}

func (m *cartModule) getCart(c *gin.Context) {
	user := currentUser(c)
	cartID, err := m.getOrCreateCart(c.Request.Context(), user.ID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	rows, err := m.pool.Query(c.Request.Context(), `
		SELECT ci.id, ci.product_variant_id, p.slug, p.name, pv.sku, pv.size, pv.color, pv.price, pv.stock, ci.quantity, ci.created_at, ci.updated_at
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
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, variantID uuid.UUID
		var productSlug, productName, sku, size, color string
		var price float64
		var stock, quantity int
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &variantID, &productSlug, &productName, &sku, &size, &color, &price, &stock, &quantity, &createdAt, &updatedAt); err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		items = append(items, gin.H{
			"id": id.String(), "product_variant_id": variantID.String(), "product_slug": productSlug, "product_name": productName, "sku": sku,
			"size": size, "color": color, "price": price, "stock": stock, "quantity": quantity, "subtotal": price * float64(quantity),
			"created_at": createdAt, "updated_at": updatedAt,
		})
	}
	response.Success(c, http.StatusOK, "Cart retrieved successfully", gin.H{"cart_id": cartID.String(), "items": items})
}

func (m *cartModule) addItem(c *gin.Context) {
	user := currentUser(c)
	var req cartItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "Validation error", validator.NormalizeErrors(err))
		return
	}
	if req.Quantity <= 0 {
		response.Error(c, http.StatusUnprocessableEntity, "Quantity must be greater than zero", nil)
		return
	}
	variantID, err := uuid.Parse(req.ProductVariantID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid variant id", nil)
		return
	}
	tx, err := m.pool.Begin(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer func() { _ = tx.Rollback(c.Request.Context()) }()
	var cartID uuid.UUID
	if err := tx.QueryRow(c.Request.Context(), `SELECT id FROM carts WHERE user_id=$1 FOR UPDATE`, user.ID).Scan(&cartID); err != nil {
		if err != pgx.ErrNoRows {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		if err := tx.QueryRow(c.Request.Context(), `INSERT INTO carts (user_id) VALUES ($1) RETURNING id`, user.ID).Scan(&cartID); err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
	}
	var stock int
	if err := tx.QueryRow(c.Request.Context(), `SELECT stock FROM product_variants WHERE id=$1 AND status='ACTIVE' FOR UPDATE`, variantID).Scan(&stock); err != nil {
		response.Error(c, http.StatusNotFound, "Variant not found", nil)
		return
	}
	var quantity int
	if err := tx.QueryRow(c.Request.Context(), `SELECT quantity FROM cart_items WHERE cart_id=$1 AND product_variant_id=$2`, cartID, variantID).Scan(&quantity); err != nil {
		if err == pgx.ErrNoRows {
			quantity = 0
		} else {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
	}
	nextQuantity := quantity + req.Quantity
	if nextQuantity > stock {
		response.Error(c, http.StatusUnprocessableEntity, "Insufficient stock", nil)
		return
	}
	if quantity == 0 {
		if _, err := tx.Exec(c.Request.Context(), `INSERT INTO cart_items (cart_id, product_variant_id, quantity) VALUES ($1,$2,$3)`, cartID, variantID, nextQuantity); err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
	} else {
		if _, err := tx.Exec(c.Request.Context(), `UPDATE cart_items SET quantity=$1, updated_at=NOW() WHERE cart_id=$2 AND product_variant_id=$3`, nextQuantity, cartID, variantID); err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.Success(c, http.StatusCreated, "Item added to cart successfully", gin.H{"cart_id": cartID.String(), "product_variant_id": variantID.String(), "quantity": nextQuantity})
}

func (m *cartModule) updateItem(c *gin.Context) {
	user := currentUser(c)
	itemID, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Quantity int `json:"quantity" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "Validation error", validator.NormalizeErrors(err))
		return
	}
	if req.Quantity <= 0 {
		response.Error(c, http.StatusUnprocessableEntity, "Quantity must be greater than zero", nil)
		return
	}
	tx, err := m.pool.Begin(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer func() { _ = tx.Rollback(c.Request.Context()) }()
	var cartID, variantID uuid.UUID
	if err := tx.QueryRow(c.Request.Context(), `
		SELECT ci.cart_id, ci.product_variant_id FROM cart_items ci
		JOIN carts c ON c.id = ci.cart_id
		WHERE ci.id=$1 AND c.user_id=$2 FOR UPDATE
	`, itemID, user.ID).Scan(&cartID, &variantID); err != nil {
		if err == pgx.ErrNoRows {
			response.Error(c, http.StatusNotFound, "Cart item not found", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	var stock int
	if err := tx.QueryRow(c.Request.Context(), `SELECT stock FROM product_variants WHERE id=$1 FOR UPDATE`, variantID).Scan(&stock); err != nil {
		response.Error(c, http.StatusNotFound, "Variant not found", nil)
		return
	}
	if req.Quantity > stock {
		response.Error(c, http.StatusUnprocessableEntity, "Insufficient stock", nil)
		return
	}
	if _, err := tx.Exec(c.Request.Context(), `UPDATE cart_items SET quantity=$1, updated_at=NOW() WHERE id=$2`, req.Quantity, itemID); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.Success(c, http.StatusOK, "Cart item updated successfully", gin.H{"cart_id": cartID.String()})
}

func (m *cartModule) deleteItem(c *gin.Context) {
	user := currentUser(c)
	itemID, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	ct, err := m.pool.Exec(c.Request.Context(), `
		DELETE FROM cart_items WHERE id=$1 AND cart_id IN (SELECT id FROM carts WHERE user_id=$2)
	`, itemID, user.ID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if ct.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "Cart item not found", nil)
		return
	}
	response.Success(c, http.StatusOK, "Cart item deleted successfully", gin.H{})
}

func (m *cartModule) clearCart(c *gin.Context) {
	user := currentUser(c)
	ct, err := m.pool.Exec(c.Request.Context(), `DELETE FROM cart_items WHERE cart_id IN (SELECT id FROM carts WHERE user_id=$1)`, user.ID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.Success(c, http.StatusOK, "Cart cleared successfully", gin.H{"deleted": ct.RowsAffected()})
}
