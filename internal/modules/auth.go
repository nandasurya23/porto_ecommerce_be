package modules

import (
	"context"
	"net/http"
	"strings"
	"time"

	"footwear-backend/internal/config"
	"footwear-backend/internal/middleware"
	"footwear-backend/internal/shared/response"
	"footwear-backend/internal/shared/security"
	"footwear-backend/internal/shared/validator"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
)

type authModule struct {
	pool        *pgxpool.Pool
	cfg         config.Config
	logger      *slog.Logger
	jwtSecret   []byte
	tokenExpiry time.Duration
}

func RegisterAuthRoutes(r *gin.RouterGroup, pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) {
	m := &authModule{pool: pool, cfg: cfg, logger: logger, jwtSecret: []byte(cfg.JWTSecret), tokenExpiry: cfg.JWTExpiresIn}
	m.ensureInternalUsers(context.Background())

	auth := r.Group("/auth")
	auth.POST("/register", m.register)
	auth.POST("/login", m.login)

	protected := r.Group("")
	protected.Use(middleware.Auth(m.jwtSecret))
	protected.GET("/auth/me", m.me)
	protected.POST("/auth/logout", m.logout)
}

type registerRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Phone    string `json:"phone"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type userResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     *string   `json:"phone,omitempty"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *authModule) register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "Validation error", validator.NormalizeErrors(err))
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		badRequest(c, "Name is required", "name")
		return
	}
	var exists bool
	if err := m.pool.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE email=$1)`, req.Email).Scan(&exists); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if exists {
		response.Error(c, http.StatusConflict, "Email already registered", nil)
		return
	}
	hashed, err := security.HashPassword(req.Password)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	var id uuid.UUID
	var createdAt, updatedAt time.Time
	var phone *string
	if strings.TrimSpace(req.Phone) != "" {
		p := strings.TrimSpace(req.Phone)
		phone = &p
	}
	err = m.pool.QueryRow(c.Request.Context(), `
		INSERT INTO users (name, email, password_hash, phone, role)
		VALUES ($1, $2, $3, $4, 'CUSTOMER')
		RETURNING id, created_at, updated_at
	`, req.Name, req.Email, hashed, phone).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	token, err := security.GenerateJWT([]byte(m.cfg.JWTSecret), m.tokenExpiry, id.String(), req.Email, "CUSTOMER")
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "User registered successfully",
		"data": gin.H{
			"user":  userResponse{ID: id.String(), Name: req.Name, Email: req.Email, Phone: phone, Role: "CUSTOMER", IsActive: true, CreatedAt: createdAt, UpdatedAt: updatedAt},
			"token": token,
		},
	})
}

func (m *authModule) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "Validation error", validator.NormalizeErrors(err))
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	row := m.pool.QueryRow(c.Request.Context(), `
		SELECT id, name, email, password_hash, phone, role, is_active, created_at, updated_at
		FROM users WHERE email=$1
	`, req.Email)
	var u userRow
	if err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Phone, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			response.Error(c, http.StatusUnauthorized, "Invalid email or password", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if !u.IsActive {
		response.Error(c, http.StatusForbidden, "User is inactive", nil)
		return
	}
	if err := security.CheckPassword(req.Password, u.PasswordHash); err != nil {
		response.Error(c, http.StatusUnauthorized, "Invalid email or password", nil)
		return
	}
	token, err := security.GenerateJWT([]byte(m.cfg.JWTSecret), m.tokenExpiry, u.ID.String(), u.Email, string(u.Role))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.Success(c, http.StatusOK, "Login successful", gin.H{
		"user":  u.toResponse(),
		"token": token,
	})
}

func (m *authModule) me(c *gin.Context) {
	user := currentUser(c)
	var u userRow
	if err := m.pool.QueryRow(c.Request.Context(), `
		SELECT id, name, email, password_hash, phone, role, is_active, created_at, updated_at
		FROM users WHERE id=$1
	`, user.ID).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Phone, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			response.Error(c, http.StatusNotFound, "User not found", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.Success(c, http.StatusOK, "User retrieved successfully", gin.H{"user": u.toResponse()})
}

func (m *authModule) logout(c *gin.Context) {
	response.Success(c, http.StatusOK, "Logout successful", gin.H{})
}

type userRow struct {
	ID           uuid.UUID
	Name         string
	Email        string
	PasswordHash string
	Phone        *string
	Role         string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (u userRow) toResponse() userResponse {
	return userResponse{
		ID:        u.ID.String(),
		Name:      u.Name,
		Email:     u.Email,
		Phone:     u.Phone,
		Role:      u.Role,
		IsActive:  u.IsActive,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func (m *authModule) ensureInternalUsers(ctx context.Context) {
	type seedUser struct {
		Name     string
		Email    string
		Role     string
		Password string
	}
	seeds := []seedUser{
		{Name: "Super Admin", Email: "superadmin@demo.com", Role: "SUPER_ADMIN", Password: "password123"},
		{Name: "Admin", Email: "admin@demo.com", Role: "ADMIN", Password: "password123"},
		{Name: "Warehouse", Email: "warehouse@demo.com", Role: "WAREHOUSE", Password: "password123"},
		{Name: "Demo Customer", Email: "customer@demo.com", Role: "CUSTOMER", Password: "password123"},
	}
	for _, seed := range seeds {
		var exists bool
		if err := m.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email=$1)`, seed.Email).Scan(&exists); err != nil || exists {
			continue
		}
		hashed, err := security.HashPassword(seed.Password)
		if err != nil {
			continue
		}
		_, _ = m.pool.Exec(ctx, `INSERT INTO users (name, email, password_hash, role) VALUES ($1, $2, $3, $4)`, seed.Name, seed.Email, hashed, seed.Role)
	}
}
