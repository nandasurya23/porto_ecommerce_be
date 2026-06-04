package modules

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
)

type catalogModule struct {
	pool      *pgxpool.Pool
	cfg       config.Config
	logger    *slog.Logger
	jwtSecret []byte
}

func RegisterCatalogRoutes(r *gin.RouterGroup, pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) {
	m := &catalogModule{pool: pool, cfg: cfg, logger: logger, jwtSecret: []byte(cfg.JWTSecret)}

	r.GET("/categories", m.publicCategories)
	r.GET("/products", m.publicProducts)
	r.GET("/products/:slug", m.publicProductDetail)

	admin := r.Group("/admin")
	admin.Use(middleware.Auth(m.jwtSecret), middleware.RequireRole("ADMIN", "SUPER_ADMIN"))
	admin.GET("/categories", m.adminCategories)
	admin.POST("/categories", m.createCategory)
	admin.PATCH("/categories/:id", m.updateCategory)
	admin.DELETE("/categories/:id", m.deleteCategory)
	admin.GET("/products", m.adminProducts)
	admin.POST("/products", m.createProduct)
	admin.PATCH("/products/:id", m.updateProduct)
	admin.DELETE("/products/:id", m.archiveProduct)
	admin.DELETE("/products/:id/permanent", m.deleteProductPermanent)
	admin.POST("/products/:id/images", m.uploadProductImage)
	admin.POST("/products/:id/variants", m.createVariant)
	admin.PATCH("/variants/:id", m.updateVariant)
	admin.GET("/inventory/logs", m.inventoryLogs)

	// inventory adjustments
	admin.PATCH("/variants/:id/stock", m.adjustStock)
}

type categoryRequest struct {
	Name        string `json:"name" binding:"required"`
	Slug        string `json:"slug" binding:"required"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
}

type productRequest struct {
	CategoryID  *string `json:"category_id"`
	Name        string  `json:"name" binding:"required"`
	Slug        string  `json:"slug" binding:"required"`
	Description string  `json:"description"`
	BasePrice   float64 `json:"base_price" binding:"required"`
	Status      string  `json:"status"`
}

type variantRequest struct {
	SKU    string  `json:"sku" binding:"required"`
	Size   string  `json:"size" binding:"required"`
	Color  string  `json:"color" binding:"required"`
	Price  float64 `json:"price" binding:"required"`
	Stock  int     `json:"stock" binding:"required"`
	Weight int     `json:"weight_gram"`
	Status string  `json:"status"`
}

func (m *catalogModule) publicCategories(c *gin.Context) {
	rows, err := m.pool.Query(c.Request.Context(), `SELECT id, name, slug, description, is_active, created_at, updated_at FROM categories WHERE is_active=true ORDER BY name ASC`)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id uuid.UUID
		var name, slug string
		var desc *string
		var active bool
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &name, &slug, &desc, &active, &createdAt, &updatedAt); err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		items = append(items, gin.H{"id": id.String(), "name": name, "slug": slug, "description": desc, "is_active": active, "created_at": createdAt, "updated_at": updatedAt})
	}
	response.Success(c, http.StatusOK, "Categories retrieved successfully", items)
}

func (m *catalogModule) adminCategories(c *gin.Context) {
	rows, err := m.pool.Query(c.Request.Context(), `SELECT id, name, slug, description, is_active, created_at, updated_at FROM categories ORDER BY created_at DESC`)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id uuid.UUID
		var name, slug string
		var desc *string
		var active bool
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &name, &slug, &desc, &active, &createdAt, &updatedAt); err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		items = append(items, gin.H{"id": id.String(), "name": name, "slug": slug, "description": desc, "is_active": active, "created_at": createdAt, "updated_at": updatedAt})
	}
	response.Success(c, http.StatusOK, "Categories retrieved successfully", items)
}

func (m *catalogModule) createCategory(c *gin.Context) {
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "Validation error", validator.NormalizeErrors(err))
		return
	}
	var id uuid.UUID
	var createdAt, updatedAt time.Time
	if err := m.pool.QueryRow(c.Request.Context(), `
		INSERT INTO categories (name, slug, description, is_active)
		VALUES ($1,$2,$3,$4)
		RETURNING id, created_at, updated_at
	`, req.Name, req.Slug, nullString(req.Description), req.IsActive).Scan(&id, &createdAt, &updatedAt); err != nil {
		response.Error(c, http.StatusConflict, "Category slug already exists", nil)
		return
	}
	response.Success(c, http.StatusCreated, "Category created successfully", gin.H{"id": id.String(), "name": req.Name, "slug": req.Slug, "description": nullString(req.Description), "is_active": req.IsActive, "created_at": createdAt, "updated_at": updatedAt})
}

func (m *catalogModule) updateCategory(c *gin.Context) {
	id, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "Validation error", validator.NormalizeErrors(err))
		return
	}
	ct, err := m.pool.Exec(c.Request.Context(), `
		UPDATE categories SET name=$1, slug=$2, description=$3, is_active=$4, updated_at=NOW()
		WHERE id=$5
	`, req.Name, req.Slug, nullString(req.Description), req.IsActive, id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if ct.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "Category not found", nil)
		return
	}
	response.Success(c, http.StatusOK, "Category updated successfully", gin.H{})
}

func (m *catalogModule) deleteCategory(c *gin.Context) {
	id, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	ct, err := m.pool.Exec(c.Request.Context(), `DELETE FROM categories WHERE id=$1`, id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if ct.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "Category not found", nil)
		return
	}
	response.Success(c, http.StatusOK, "Category deleted successfully", gin.H{})
}

func (m *catalogModule) publicProducts(c *gin.Context) { m.productList(c, true) }
func (m *catalogModule) adminProducts(c *gin.Context)  { m.productList(c, false) }

func (m *catalogModule) productList(c *gin.Context, onlyPublished bool) {
	page, limit := pagination.Parse(c.Query("page"), c.Query("limit"), 12)
	offset := (page - 1) * limit

	query := `
		SELECT p.id, COALESCE(p.category_id::text, ''), COALESCE(c.name, ''), p.name, p.slug, p.description, p.base_price, p.status, p.created_at, p.updated_at
		FROM products p
		LEFT JOIN categories c ON c.id = p.category_id
	`
	args := []any{}
	if onlyPublished {
		query += ` WHERE p.status='PUBLISHED'`
	}
	query += ` ORDER BY p.created_at DESC LIMIT $1 OFFSET $2`
	args = append(args, limit, offset)
	rows, err := m.pool.Query(c.Request.Context(), query, args...)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id uuid.UUID
		var categoryID string
		var categoryName string
		var name, slug string
		var desc *string
		var basePrice float64
		var status string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &categoryID, &categoryName, &name, &slug, &desc, &basePrice, &status, &createdAt, &updatedAt); err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		items = append(items, gin.H{"id": id.String(), "category_id": blankOrPointer(categoryID), "category_name": blankOrPointer(categoryName), "name": name, "slug": slug, "description": desc, "base_price": basePrice, "status": status, "created_at": createdAt, "updated_at": updatedAt})
	}
	var total int
	countQuery := `SELECT COUNT(*) FROM products`
	if onlyPublished {
		countQuery += ` WHERE status='PUBLISHED'`
	}
	if err := m.pool.QueryRow(c.Request.Context(), countQuery).Scan(&total); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.SuccessWithMeta(c, http.StatusOK, "Products retrieved successfully", items, pagination.Meta{Page: page, Limit: limit, Total: total, TotalPages: totalPages(total, limit)})
}

func (m *catalogModule) publicProductDetail(c *gin.Context) {
	slug := c.Param("slug")
	product, err := m.productDetailBySlug(c.Request.Context(), slug, true)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Product not found", nil)
		return
	}
	response.Success(c, http.StatusOK, "Product retrieved successfully", product)
}

func (m *catalogModule) createProduct(c *gin.Context) {
	var req productRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "Validation error", validator.NormalizeErrors(err))
		return
	}
	var id uuid.UUID
	var createdAt, updatedAt time.Time
	if err := m.pool.QueryRow(c.Request.Context(), `
		INSERT INTO products (category_id, name, slug, description, base_price, status)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, created_at, updated_at
	`, nullUUID(req.CategoryID), req.Name, req.Slug, nullString(req.Description), req.BasePrice, statusOr(req.Status, "DRAFT")).Scan(&id, &createdAt, &updatedAt); err != nil {
		response.Error(c, http.StatusConflict, "Product slug already exists", nil)
		return
	}
	response.Success(c, http.StatusCreated, "Product created successfully", gin.H{"id": id.String(), "created_at": createdAt, "updated_at": updatedAt})
}

func (m *catalogModule) updateProduct(c *gin.Context) {
	id, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	var req productRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "Validation error", validator.NormalizeErrors(err))
		return
	}
	ct, err := m.pool.Exec(c.Request.Context(), `
		UPDATE products SET category_id=$1, name=$2, slug=$3, description=$4, base_price=$5, status=$6, updated_at=NOW()
		WHERE id=$7
	`, nullUUID(req.CategoryID), req.Name, req.Slug, nullString(req.Description), req.BasePrice, statusOr(req.Status, "DRAFT"), id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if ct.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "Product not found", nil)
		return
	}
	response.Success(c, http.StatusOK, "Product updated successfully", gin.H{})
}

func (m *catalogModule) archiveProduct(c *gin.Context) {
	id, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	ct, err := m.pool.Exec(c.Request.Context(), `UPDATE products SET status='ARCHIVED', updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if ct.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "Product not found", nil)
		return
	}
	response.Success(c, http.StatusOK, "Product archived successfully", gin.H{})
}

func (m *catalogModule) deleteProductPermanent(c *gin.Context) {
	id, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}

	tx, err := m.pool.Begin(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer func() {
		_ = tx.Rollback(c.Request.Context())
	}()

	if _, err := tx.Exec(c.Request.Context(), `
		DELETE FROM cart_items
		WHERE product_variant_id IN (SELECT id FROM product_variants WHERE product_id=$1)
	`, id); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}

	if _, err := tx.Exec(c.Request.Context(), `
		DELETE FROM inventory_logs
		WHERE product_variant_id IN (SELECT id FROM product_variants WHERE product_id=$1)
	`, id); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}

	ct, err := tx.Exec(c.Request.Context(), `DELETE FROM products WHERE id=$1`, id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if ct.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "Product not found", nil)
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}

	response.Success(c, http.StatusOK, "Product permanently deleted successfully", gin.H{})
}

func (m *catalogModule) uploadProductImage(c *gin.Context) {
	productID, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Image file is required", nil)
		return
	}
	defer file.Close()
	if header.Size > m.cfg.MaxUploadSizeMB*1024*1024 {
		response.Error(c, http.StatusRequestEntityTooLarge, "File too large", nil)
		return
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedImageExt(ext) {
		response.Error(c, http.StatusBadRequest, "Unsupported image extension", nil)
		return
	}
	sniff := make([]byte, 512)
	n, _ := file.Read(sniff)
	contentType := http.DetectContentType(sniff[:n])
	if !allowedImageContentType(contentType) {
		response.Error(c, http.StatusBadRequest, "Unsupported image content type", nil)
		return
	}
	if err := os.MkdirAll(filepath.Join(m.cfg.UploadDir, "products", productID.String()), 0o755); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if ext == "" {
		ext = ".jpg"
	}
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	dst := filepath.Join(m.cfg.UploadDir, "products", productID.String(), filename)
	out, err := os.Create(dst)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, io.MultiReader(bytes.NewReader(sniff[:n]), file)); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	isPrimary := c.PostForm("is_primary") == "true"
	alt := c.PostForm("alt_text")
	sortOrder, _ := strconv.Atoi(c.PostForm("sort_order"))
	if isPrimary {
		_, _ = m.pool.Exec(c.Request.Context(), `UPDATE product_images SET is_primary=false WHERE product_id=$1`, productID)
	}
	imageURL := "/uploads/products/" + productID.String() + "/" + filename
	var id uuid.UUID
	if err := m.pool.QueryRow(c.Request.Context(), `
		INSERT INTO product_images (product_id, image_url, alt_text, sort_order, is_primary)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id
	`, productID, imageURL, nullString(alt), sortOrder, isPrimary).Scan(&id); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.Success(c, http.StatusCreated, "Image uploaded successfully", gin.H{"id": id.String(), "image_url": imageURL, "is_primary": isPrimary})
}

func allowedImageExt(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
		return true
	default:
		return false
	}
}

func allowedImageContentType(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func (m *catalogModule) createVariant(c *gin.Context) {
	productID, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	var req variantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "Validation error", validator.NormalizeErrors(err))
		return
	}
	var id uuid.UUID
	if err := m.pool.QueryRow(c.Request.Context(), `
		INSERT INTO product_variants (product_id, sku, size, color, price, stock, weight_gram, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id
	`, productID, req.SKU, req.Size, req.Color, req.Price, req.Stock, req.Weight, statusOr(req.Status, "ACTIVE")).Scan(&id); err != nil {
		response.Error(c, http.StatusConflict, "Variant SKU already exists", nil)
		return
	}
	response.Success(c, http.StatusCreated, "Variant created successfully", gin.H{"id": id.String()})
}

func (m *catalogModule) updateVariant(c *gin.Context) {
	id, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	var req variantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "Validation error", validator.NormalizeErrors(err))
		return
	}
	ct, err := m.pool.Exec(c.Request.Context(), `
		UPDATE product_variants SET sku=$1, size=$2, color=$3, price=$4, stock=$5, weight_gram=$6, status=$7, updated_at=NOW()
		WHERE id=$8
	`, req.SKU, req.Size, req.Color, req.Price, req.Stock, req.Weight, statusOr(req.Status, "ACTIVE"), id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if ct.RowsAffected() == 0 {
		response.Error(c, http.StatusNotFound, "Variant not found", nil)
		return
	}
	response.Success(c, http.StatusOK, "Variant updated successfully", gin.H{})
}

func (m *catalogModule) adjustStock(c *gin.Context) {
	id, ok := mustUUIDParam(c, "id")
	if !ok {
		return
	}
	var body struct {
		Quantity int    `json:"quantity" binding:"required"`
		Type     string `json:"type" binding:"required"`
		Note     string `json:"note"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "Validation error", validator.NormalizeErrors(err))
		return
	}
	ctx := c.Request.Context()
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var stock int
	if err := tx.QueryRow(ctx, `SELECT stock FROM product_variants WHERE id=$1 FOR UPDATE`, id).Scan(&stock); err != nil {
		response.Error(c, http.StatusNotFound, "Variant not found", nil)
		return
	}
	next := stock + body.Quantity
	if next < 0 {
		response.Error(c, http.StatusUnprocessableEntity, "Insufficient stock", nil)
		return
	}
	var productID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT product_id FROM product_variants WHERE id=$1`, id).Scan(&productID); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	if _, err := tx.Exec(ctx, `UPDATE product_variants SET stock=$1, status=CASE WHEN $1=0 THEN 'OUT_OF_STOCK'::variant_status ELSE 'ACTIVE'::variant_status END, updated_at=NOW() WHERE id=$2`, next, id); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	_, _ = tx.Exec(ctx, `
		INSERT INTO inventory_logs (product_variant_id, type, quantity, previous_stock, current_stock, note, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, id, body.Type, body.Quantity, stock, next, body.Note, currentUser(c).ID)
	if err := tx.Commit(ctx); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.Success(c, http.StatusOK, "Stock adjusted successfully", gin.H{"product_id": productID.String(), "previous_stock": stock, "current_stock": next})
}

func (m *catalogModule) inventoryLogs(c *gin.Context) {
	page, limit := pagination.Parse(c.Query("page"), c.Query("limit"), 20)
	offset := (page - 1) * limit
	rows, err := m.pool.Query(c.Request.Context(), `
		SELECT il.id, il.product_variant_id, pv.sku, il.type, il.quantity, il.previous_stock, il.current_stock, COALESCE(il.note, ''), COALESCE(il.created_by::text, ''), il.created_at
		FROM inventory_logs il
		JOIN product_variants pv ON pv.id = il.product_variant_id
		ORDER BY il.created_at DESC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id, variantID uuid.UUID
		var sku, typ string
		var qty, prev, curr int
		var note string
		var createdBy string
		var createdAt time.Time
		if err := rows.Scan(&id, &variantID, &sku, &typ, &qty, &prev, &curr, &note, &createdBy, &createdAt); err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
			return
		}
		items = append(items, gin.H{"id": id.String(), "product_variant_id": variantID.String(), "sku": sku, "type": typ, "quantity": qty, "previous_stock": prev, "current_stock": curr, "note": blankOrPointer(note), "created_by": blankOrPointer(createdBy), "created_at": createdAt})
	}
	var total int
	if err := m.pool.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM inventory_logs`).Scan(&total); err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	response.SuccessWithMeta(c, http.StatusOK, "Inventory logs retrieved successfully", items, pagination.Meta{Page: page, Limit: limit, Total: total, TotalPages: totalPages(total, limit)})
}

func (m *catalogModule) productDetailBySlug(ctx context.Context, slug string, onlyPublished bool) (gin.H, error) {
	query := `
		SELECT p.id, COALESCE(p.category_id::text, ''), COALESCE(c.name, ''), p.name, p.slug, p.description, p.base_price, p.status, p.created_at, p.updated_at
		FROM products p
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE p.slug=$1
	`
	if onlyPublished {
		query += ` AND p.status='PUBLISHED'`
	}
	var id uuid.UUID
	var categoryID string
	var categoryName string
	var name, slugValue string
	var desc *string
	var basePrice float64
	var status string
	var createdAt, updatedAt time.Time
	if err := m.pool.QueryRow(ctx, query, slug).Scan(&id, &categoryID, &categoryName, &name, &slugValue, &desc, &basePrice, &status, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	imgRows, err := m.pool.Query(ctx, `SELECT id, image_url, alt_text, sort_order, is_primary FROM product_images WHERE product_id=$1 ORDER BY sort_order ASC, created_at ASC`, id)
	if err != nil {
		return nil, err
	}
	defer imgRows.Close()
	images := make([]gin.H, 0)
	for imgRows.Next() {
		var imgID uuid.UUID
		var url string
		var alt *string
		var order int
		var primary bool
		if err := imgRows.Scan(&imgID, &url, &alt, &order, &primary); err != nil {
			return nil, err
		}
		images = append(images, gin.H{"id": imgID.String(), "image_url": url, "alt_text": alt, "sort_order": order, "is_primary": primary})
	}
	variants, err := m.loadVariants(ctx, id)
	if err != nil {
		return nil, err
	}
	return gin.H{
		"id": id.String(), "category_id": blankOrPointer(categoryID), "category_name": blankOrPointer(categoryName), "name": name, "slug": slugValue,
		"description": desc, "base_price": basePrice, "status": status, "images": images, "variants": variants, "created_at": createdAt, "updated_at": updatedAt,
	}, nil
}

func (m *catalogModule) loadVariants(ctx context.Context, productID uuid.UUID) ([]gin.H, error) {
	rows, err := m.pool.Query(ctx, `SELECT id, sku, size, color, price, stock, weight_gram, status FROM product_variants WHERE product_id=$1 ORDER BY created_at ASC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]gin.H, 0)
	for rows.Next() {
		var id uuid.UUID
		var sku, size, color, status string
		var price float64
		var stock, weight int
		if err := rows.Scan(&id, &sku, &size, &color, &price, &stock, &weight, &status); err != nil {
			return nil, err
		}
		out = append(out, gin.H{"id": id.String(), "sku": sku, "size": size, "color": color, "price": price, "stock": stock, "weight_gram": weight, "status": status})
	}
	return out, nil
}

func nullString(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	s := strings.TrimSpace(v)
	return &s
}

func nullUUID(v *string) *uuid.UUID {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil
	}
	id, err := uuid.Parse(strings.TrimSpace(*v))
	if err != nil {
		return nil
	}
	return &id
}

func blankOrPointer(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	s := strings.TrimSpace(v)
	return &s
}

func statusOr(v, fallback string) string {
	v = strings.TrimSpace(strings.ToUpper(v))
	if v == "" {
		return fallback
	}
	return v
}

func totalPages(total, limit int) int {
	if limit <= 0 {
		return 0
	}
	if total == 0 {
		return 0
	}
	return (total + limit - 1) / limit
}
