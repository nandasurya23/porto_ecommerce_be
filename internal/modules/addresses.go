package modules

import (
	"net/http"
	"time"

	"footwear-backend/internal/config"
	"footwear-backend/internal/middleware"
	"footwear-backend/internal/shared/response"
	"footwear-backend/internal/shared/validator"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
)

type addressesModule struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func RegisterAddressRoutes(r *gin.RouterGroup, pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) {
	m := &addressesModule{pool: pool, logger: logger}
	group := r.Group("/addresses")
	group.Use(middleware.Auth([]byte(cfg.JWTSecret)))
	group.GET("", m.list)
	group.POST("", m.create)
	group.PATCH("/:id", m.update)
	group.DELETE("/:id", m.delete)
}

type addressRequest struct {
	ReceiverName string `json:"receiver_name" binding:"required"`
	Phone        string `json:"phone" binding:"required"`
	Province     string `json:"province" binding:"required"`
	City         string `json:"city" binding:"required"`
	District     string `json:"district"`
	PostalCode   string `json:"postal_code"`
	FullAddress  string `json:"full_address" binding:"required"`
	IsDefault    bool   `json:"is_default"`
}

type addressResponse struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	ReceiverName string    `json:"receiver_name"`
	Phone        string    `json:"phone"`
	Province     string    `json:"province"`
	City         string    `json:"city"`
	District     *string   `json:"district,omitempty"`
	PostalCode   *string   `json:"postal_code,omitempty"`
	FullAddress  string    `json:"full_address"`
	IsDefault    bool      `json:"is_default"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (m *addressesModule) list(c *gin.Context) {
	user := currentUser(c)
	rows, err := m.pool.Query(c.Request.Context(), `
		SELECT id, user_id, receiver_name, phone, province, city, district, postal_code, full_address, is_default, created_at, updated_at
		FROM addresses WHERE user_id=$1 ORDER BY is_default DESC, created_at DESC
	`, user.ID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer rows.Close()
	items := make([]addressResponse, 0)
	for rows.Next() {
		var a addressRow
		if err := rows.Scan(&a.ID, &a.UserID, &a.ReceiverName, &a.Phone, &a.Province, &a.City, &a.District, &a.PostalCode, &a.FullAddress, &a.IsDefault, &a.CreatedAt, &a.UpdatedAt); err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		items = append(items, a.toResponse())
	}
	response.Success(c, http.StatusOK, "Addresses retrieved successfully", items)
}

func (m *addressesModule) create(c *gin.Context) {
	user := currentUser(c)
	var req addressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "Validation error", validator.NormalizeErrors(err))
		return
	}
	if req.IsDefault {
		_, _ = m.pool.Exec(c.Request.Context(), `UPDATE addresses SET is_default=false, updated_at=NOW() WHERE user_id=$1`, user.ID)
	}
	var id uuid.UUID
	var createdAt, updatedAt time.Time
	var district, postal *string
	if req.District != "" {
		district = &req.District
	}
	if req.PostalCode != "" {
		postal = &req.PostalCode
	}
	err := m.pool.QueryRow(c.Request.Context(), `
		INSERT INTO addresses (user_id, receiver_name, phone, province, city, district, postal_code, full_address, is_default)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, created_at, updated_at
	`, user.ID, req.ReceiverName, req.Phone, req.Province, req.City, district, postal, req.FullAddress, req.IsDefault).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.Success(c, http.StatusCreated, "Address created successfully", addressResponse{
		ID: id.String(), UserID: user.ID, ReceiverName: req.ReceiverName, Phone: req.Phone, Province: req.Province, City: req.City,
		District: district, PostalCode: postal, FullAddress: req.FullAddress, IsDefault: req.IsDefault, CreatedAt: createdAt, UpdatedAt: updatedAt,
	})
}

func (m *addressesModule) update(c *gin.Context) {
	user := currentUser(c)
	id, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	var req addressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "Validation error", validator.NormalizeErrors(err))
		return
	}
	if req.IsDefault {
		_, _ = m.pool.Exec(c.Request.Context(), `UPDATE addresses SET is_default=false, updated_at=NOW() WHERE user_id=$1`, user.ID)
	}
	var district, postal *string
	if req.District != "" {
		district = &req.District
	}
	if req.PostalCode != "" {
		postal = &req.PostalCode
	}
	ct, err := m.pool.Exec(c.Request.Context(), `
		UPDATE addresses
		SET receiver_name=$1, phone=$2, province=$3, city=$4, district=$5, postal_code=$6, full_address=$7, is_default=$8, updated_at=NOW()
		WHERE id=$9 AND user_id=$10
	`, req.ReceiverName, req.Phone, req.Province, req.City, district, postal, req.FullAddress, req.IsDefault, id, user.ID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if ct.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "Address not found", nil)
		return
	}
	response.Success(c, http.StatusOK, "Address updated successfully", gin.H{})
}

func (m *addressesModule) delete(c *gin.Context) {
	user := currentUser(c)
	id, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	ct, err := m.pool.Exec(c.Request.Context(), `DELETE FROM addresses WHERE id=$1 AND user_id=$2`, id, user.ID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if ct.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "Address not found", nil)
		return
	}
	response.Success(c, http.StatusOK, "Address deleted successfully", gin.H{})
}

type addressRow struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	ReceiverName string
	Phone        string
	Province     string
	City         string
	District     *string
	PostalCode   *string
	FullAddress  string
	IsDefault    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (a addressRow) toResponse() addressResponse {
	return addressResponse{
		ID: a.ID.String(), UserID: a.UserID.String(), ReceiverName: a.ReceiverName, Phone: a.Phone, Province: a.Province, City: a.City,
		District: a.District, PostalCode: a.PostalCode, FullAddress: a.FullAddress, IsDefault: a.IsDefault, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}
