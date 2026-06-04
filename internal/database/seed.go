package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"footwear-backend/internal/config"
	"footwear-backend/internal/shared/security"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type demoUserSeed struct {
	Name     string
	Email    string
	Role     string
	Password string
	Phone    *string
}

type demoCategorySeed struct {
	Name        string
	Slug        string
	Description string
}

type demoProductSeed struct {
	Name        string
	Slug        string
	Category    string
	Description string
	BasePrice   float64
	Status      string
	Images      []demoImageSeed
	Variants    []demoVariantSeed
}

type demoImageSeed struct {
	FileName  string
	AltText   string
	SortOrder int
	IsPrimary bool
}

type demoVariantSeed struct {
	SKU        string
	Size       string
	Color      string
	Price      float64
	Stock      int
	WeightGram int
	Status     string
}

type demoOrderLineSeed struct {
	VariantSKU string
	Quantity   int
}

type demoOrderSeed struct {
	OrderNumber   string
	PaymentCode   string
	PaymentMethod string
	PaymentStatus string
	OrderStatus   string
	AddressEmail  string
	CreatedAt     time.Time
	Lines         []demoOrderLineSeed
	Shipment      *demoShipmentSeed
}

type demoShipmentSeed struct {
	Courier        string
	ServiceName    string
	TrackingNumber string
	Status         string
	ShippedAt      *time.Time
	DeliveredAt    *time.Time
}

func (db *DB) SeedDemoData(ctx context.Context, cfg config.Config, logger interface{ Info(string, ...any) }) error {
	assetsDir := filepath.Join(cfg.UploadDir, "seed")
	if err := ensureDemoAssets(assetsDir); err != nil {
		return err
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC().Truncate(time.Second)

	users := []demoUserSeed{
		{Name: "Super Admin", Email: "superadmin@demo.com", Role: "SUPER_ADMIN", Password: "password123"},
		{Name: "Admin", Email: "admin@demo.com", Role: "ADMIN", Password: "password123"},
		{Name: "Warehouse", Email: "warehouse@demo.com", Role: "WAREHOUSE", Password: "password123"},
		{Name: "Demo Customer", Email: "customer@demo.com", Role: "CUSTOMER", Password: "password123", Phone: ptrString("+6281234567890")},
	}
	userIDs := make(map[string]uuid.UUID, len(users))
	for _, seed := range users {
		id, err := ensureUser(ctx, tx, seed)
		if err != nil {
			return err
		}
		userIDs[seed.Email] = id
	}

	categories := []demoCategorySeed{
		{Name: "Running Shoes", Slug: "running-shoes", Description: "Shoes for running"},
		{Name: "Lifestyle Shoes", Slug: "lifestyle-shoes", Description: "Daily casual shoes"},
		{Name: "Training Shoes", Slug: "training-shoes", Description: "Shoes for training"},
		{Name: "Football Shoes", Slug: "football-shoes", Description: "Shoes for football"},
		{Name: "Sandals", Slug: "sandals", Description: "Casual sandals"},
		{Name: "Limited Edition", Slug: "limited-edition", Description: "Limited footwear releases"},
	}
	categoryIDs := make(map[string]uuid.UUID, len(categories))
	for _, seed := range categories {
		id, err := ensureCategory(ctx, tx, seed, now)
		if err != nil {
			return err
		}
		categoryIDs[seed.Slug] = id
	}

	products := []demoProductSeed{
		{
			Name:        "AeroFlex Runner",
			Slug:        "aeroflex-runner",
			Category:    "running-shoes",
			Description: "Lightweight running shoes for daily mileage and tempo sessions.",
			BasePrice:   1299000,
			Status:      "PUBLISHED",
			Images: []demoImageSeed{
				{FileName: "aeroflex-runner-1.svg", AltText: "AeroFlex Runner front view", SortOrder: 0, IsPrimary: true},
				{FileName: "aeroflex-runner-2.svg", AltText: "AeroFlex Runner side view", SortOrder: 1, IsPrimary: false},
			},
			Variants: []demoVariantSeed{
				{SKU: "AFR-40-BLK", Size: "40", Color: "Black", Price: 1299000, Stock: 12, WeightGram: 420, Status: "ACTIVE"},
				{SKU: "AFR-41-GRY", Size: "41", Color: "Grey", Price: 1299000, Stock: 4, WeightGram: 425, Status: "ACTIVE"},
			},
		},
		{
			Name:        "Urban Glide",
			Slug:        "urban-glide",
			Category:    "lifestyle-shoes",
			Description: "Clean everyday sneakers for commute, work, and casual wear.",
			BasePrice:   899000,
			Status:      "PUBLISHED",
			Images: []demoImageSeed{
				{FileName: "urban-glide-1.svg", AltText: "Urban Glide front view", SortOrder: 0, IsPrimary: true},
				{FileName: "urban-glide-2.svg", AltText: "Urban Glide detail view", SortOrder: 1, IsPrimary: false},
			},
			Variants: []demoVariantSeed{
				{SKU: "UGL-39-WHT", Size: "39", Color: "White", Price: 899000, Stock: 15, WeightGram: 390, Status: "ACTIVE"},
				{SKU: "UGL-42-NAV", Size: "42", Color: "Navy", Price: 899000, Stock: 5, WeightGram: 395, Status: "ACTIVE"},
			},
		},
		{
			Name:        "Core Trainer",
			Slug:        "core-trainer",
			Category:    "training-shoes",
			Description: "Stable training shoes with a firmer platform for gym sessions.",
			BasePrice:   1099000,
			Status:      "PUBLISHED",
			Images: []demoImageSeed{
				{FileName: "core-trainer-1.svg", AltText: "Core Trainer front view", SortOrder: 0, IsPrimary: true},
				{FileName: "core-trainer-2.svg", AltText: "Core Trainer outsole view", SortOrder: 1, IsPrimary: false},
			},
			Variants: []demoVariantSeed{
				{SKU: "CTR-40-BLK", Size: "40", Color: "Black", Price: 1099000, Stock: 9, WeightGram: 460, Status: "ACTIVE"},
				{SKU: "CTR-42-RED", Size: "42", Color: "Red", Price: 1099000, Stock: 0, WeightGram: 465, Status: "OUT_OF_STOCK"},
			},
		},
		{
			Name:        "Strike Pro TF",
			Slug:        "strike-pro-tf",
			Category:    "football-shoes",
			Description: "Turf football boots for fast turns and responsive grip.",
			BasePrice:   1499000,
			Status:      "PUBLISHED",
			Images: []demoImageSeed{
				{FileName: "strike-pro-tf-1.svg", AltText: "Strike Pro TF front view", SortOrder: 0, IsPrimary: true},
				{FileName: "strike-pro-tf-2.svg", AltText: "Strike Pro TF heel view", SortOrder: 1, IsPrimary: false},
			},
			Variants: []demoVariantSeed{
				{SKU: "SPT-41-YEL", Size: "41", Color: "Yellow", Price: 1499000, Stock: 7, WeightGram: 440, Status: "ACTIVE"},
				{SKU: "SPT-42-BLK", Size: "42", Color: "Black", Price: 1499000, Stock: 3, WeightGram: 445, Status: "ACTIVE"},
			},
		},
		{
			Name:        "Coast Slide",
			Slug:        "coast-slide",
			Category:    "sandals",
			Description: "Comfortable slides for home, travel, and recovery days.",
			BasePrice:   299000,
			Status:      "PUBLISHED",
			Images: []demoImageSeed{
				{FileName: "coast-slide-1.svg", AltText: "Coast Slide top view", SortOrder: 0, IsPrimary: true},
				{FileName: "coast-slide-2.svg", AltText: "Coast Slide side view", SortOrder: 1, IsPrimary: false},
			},
			Variants: []demoVariantSeed{
				{SKU: "CSD-M-GRN", Size: "M", Color: "Green", Price: 299000, Stock: 20, WeightGram: 220, Status: "ACTIVE"},
				{SKU: "CSD-L-BRN", Size: "L", Color: "Brown", Price: 299000, Stock: 8, WeightGram: 225, Status: "ACTIVE"},
			},
		},
	}
	variantIDs := make(map[string]uuid.UUID)
	for _, seed := range products {
		productID, err := ensureProduct(ctx, tx, seed, categoryIDs[seed.Category], now)
		if err != nil {
			return err
		}
		if err := ensureProductImages(ctx, tx, productID, seed.Images); err != nil {
			return err
		}
		for _, variant := range seed.Variants {
			variantID, err := ensureVariant(ctx, tx, productID, variant, now)
			if err != nil {
				return err
			}
			variantIDs[variant.SKU] = variantID
		}
	}

	customerID := userIDs["customer@demo.com"]
	addressID, err := ensureAddress(ctx, tx, customerID, now)
	if err != nil {
		return err
	}

	if err := ensureCart(ctx, tx, customerID, []cartItemSeed{
		{VariantSKU: "AFR-40-BLK", Quantity: 1},
		{VariantSKU: "UGL-39-WHT", Quantity: 2},
	}, variantIDs, now); err != nil {
		return err
	}

	orders := []demoOrderSeed{
		{
			OrderNumber:   "ORD-DEMO-0001",
			PaymentCode:   "PAY-DEMO-0001",
			PaymentMethod: "BANK_TRANSFER",
			PaymentStatus: "PAID",
			OrderStatus:   "DELIVERED",
			AddressEmail:  "customer@demo.com",
			CreatedAt:     now.AddDate(0, 0, -2),
			Lines: []demoOrderLineSeed{
				{VariantSKU: "AFR-40-BLK", Quantity: 1},
				{VariantSKU: "CTR-40-BLK", Quantity: 1},
			},
			Shipment: &demoShipmentSeed{
				Courier:        "JNE",
				ServiceName:    "YES",
				TrackingNumber: "JNE-DEMO-0001",
				Status:         "DELIVERED",
				ShippedAt:      ptrTime(now.AddDate(0, 0, -2).Add(6 * time.Hour)),
				DeliveredAt:    ptrTime(now.AddDate(0, 0, -1).Add(9 * time.Hour)),
			},
		},
		{
			OrderNumber:   "ORD-DEMO-0002",
			PaymentCode:   "PAY-DEMO-0002",
			PaymentMethod: "QRIS_SIMULATION",
			PaymentStatus: "PAID",
			OrderStatus:   "PROCESSING",
			AddressEmail:  "customer@demo.com",
			CreatedAt:     now.AddDate(0, 0, -1),
			Lines: []demoOrderLineSeed{
				{VariantSKU: "SPT-41-YEL", Quantity: 1},
			},
			Shipment: &demoShipmentSeed{
				Courier:        "SiCepat",
				ServiceName:    "REG",
				TrackingNumber: "SICEPAT-DEMO-0002",
				Status:         "IN_TRANSIT",
				ShippedAt:      ptrTime(now.AddDate(0, 0, -1).Add(4 * time.Hour)),
			},
		},
		{
			OrderNumber:   "ORD-DEMO-0003",
			PaymentCode:   "PAY-DEMO-0003",
			PaymentMethod: "VIRTUAL_ACCOUNT",
			PaymentStatus: "PENDING",
			OrderStatus:   "PENDING_PAYMENT",
			AddressEmail:  "customer@demo.com",
			CreatedAt:     now,
			Lines: []demoOrderLineSeed{
				{VariantSKU: "CSD-M-GRN", Quantity: 1},
			},
		},
	}
	for _, seed := range orders {
		if err := ensureOrder(ctx, tx, seed, customerID, addressID, variantIDs, now); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	logger.Info("demo seed ensured")
	return nil
}

type cartItemSeed struct {
	VariantSKU string
	Quantity   int
}

func ensureUser(ctx context.Context, tx pgx.Tx, seed demoUserSeed) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO users (name, email, password_hash, phone, role)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (email) DO NOTHING
		RETURNING id
	`, seed.Name, seed.Email, mustDemoPasswordHash(seed.Password), seed.Phone, seed.Role).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return uuid.UUID{}, err
	}
	return lookupUUID(ctx, tx, `SELECT id FROM users WHERE email=$1`, seed.Email)
}

func ensureCategory(ctx context.Context, tx pgx.Tx, seed demoCategorySeed, now time.Time) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO categories (name, slug, description, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, TRUE, $4, $4)
		ON CONFLICT (slug) DO NOTHING
		RETURNING id
	`, seed.Name, seed.Slug, seed.Description, now).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return uuid.UUID{}, err
	}
	return lookupUUID(ctx, tx, `SELECT id FROM categories WHERE slug=$1`, seed.Slug)
}

func ensureProduct(ctx context.Context, tx pgx.Tx, seed demoProductSeed, categoryID uuid.UUID, now time.Time) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO products (category_id, name, slug, description, base_price, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		ON CONFLICT (slug) DO NOTHING
		RETURNING id
	`, categoryID, seed.Name, seed.Slug, seed.Description, seed.BasePrice, seed.Status, now).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return uuid.UUID{}, err
	}
	return lookupUUID(ctx, tx, `SELECT id FROM products WHERE slug=$1`, seed.Slug)
}

func ensureProductImages(ctx context.Context, tx pgx.Tx, productID uuid.UUID, images []demoImageSeed) error {
	for _, img := range images {
		imageURL := "/uploads/seed/" + img.FileName
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM product_images WHERE product_id=$1 AND image_url=$2)`, productID, imageURL).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		var id uuid.UUID
		err := tx.QueryRow(ctx, `
			INSERT INTO product_images (product_id, image_url, alt_text, sort_order, is_primary)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING
			RETURNING id
		`, productID, imageURL, optionalString(img.AltText), img.SortOrder, img.IsPrimary).Scan(&id)
		if err == nil {
			continue
		}
		if err != pgx.ErrNoRows {
			return err
		}
	}
	return nil
}

func ensureVariant(ctx context.Context, tx pgx.Tx, productID uuid.UUID, seed demoVariantSeed, now time.Time) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO product_variants (product_id, sku, size, color, price, stock, weight_gram, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		ON CONFLICT (sku) DO NOTHING
		RETURNING id
	`, productID, seed.SKU, seed.Size, seed.Color, seed.Price, seed.Stock, seed.WeightGram, seed.Status, now).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return uuid.UUID{}, err
	}
	return lookupUUID(ctx, tx, `SELECT id FROM product_variants WHERE sku=$1`, seed.SKU)
}

func ensureAddress(ctx context.Context, tx pgx.Tx, userID uuid.UUID, now time.Time) (uuid.UUID, error) {
	var existing uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM addresses WHERE user_id=$1 AND is_default=true ORDER BY created_at ASC LIMIT 1`, userID).Scan(&existing)
	if err == nil {
		return existing, nil
	}
	if err != pgx.ErrNoRows {
		return uuid.UUID{}, err
	}
	var id uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO addresses (user_id, receiver_name, phone, province, city, district, postal_code, full_address, is_default, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, TRUE, $9, $9)
		ON CONFLICT DO NOTHING
		RETURNING id
	`, userID, "Demo Customer", "+6281234567890", "South Sulawesi", "Makassar", "Panakkukang", "90231", "Jl. Demo No. 88, Makassar, Sulawesi Selatan", now).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return uuid.UUID{}, err
	}
	return lookupUUID(ctx, tx, `SELECT id FROM addresses WHERE user_id=$1 AND is_default=true`, userID)
}

func ensureCart(ctx context.Context, tx pgx.Tx, userID uuid.UUID, items []cartItemSeed, variantIDs map[string]uuid.UUID, now time.Time) error {
	var cartID uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO carts (user_id, created_at, updated_at)
		VALUES ($1, $2, $2)
		ON CONFLICT (user_id) DO NOTHING
		RETURNING id
	`, userID, now).Scan(&cartID)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if err == pgx.ErrNoRows {
		cartID, err = lookupUUID(ctx, tx, `SELECT id FROM carts WHERE user_id=$1`, userID)
		if err != nil {
			return err
		}
	}
	for _, item := range items {
		variantID := variantIDs[item.VariantSKU]
		if variantID == uuid.Nil {
			return fmt.Errorf("variant not found for cart seed: %s", item.VariantSKU)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO cart_items (cart_id, product_variant_id, quantity, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $4)
			ON CONFLICT (cart_id, product_variant_id) DO NOTHING
		`, cartID, variantID, item.Quantity, now); err != nil {
			return err
		}
	}
	return nil
}

func ensureOrder(ctx context.Context, tx pgx.Tx, seed demoOrderSeed, customerID, addressID uuid.UUID, variantIDs map[string]uuid.UUID, now time.Time) error {
	var orderID uuid.UUID
	var address any
	if addressID != uuid.Nil {
		address = addressID
	}
	subtotal, shipping, total, lines, err := buildOrderTotals(ctx, tx, seed.Lines, variantIDs)
	if err != nil {
		return err
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (
			user_id, address_id, order_number, subtotal, shipping_cost, discount_amount, total_amount, payment_status, order_status, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, 0, $6, $7, $8, $9, $9)
		ON CONFLICT (order_number) DO NOTHING
		RETURNING id
	`, customerID, address, seed.OrderNumber, subtotal, shipping, total, seed.PaymentStatus, seed.OrderStatus, seed.CreatedAt).Scan(&orderID)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if err == pgx.ErrNoRows {
		orderID, err = lookupUUID(ctx, tx, `SELECT id FROM orders WHERE order_number=$1`, seed.OrderNumber)
		if err != nil {
			return err
		}
		return nil
	}

	for _, line := range lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_items (
				order_id, product_variant_id, product_name_snapshot, product_slug_snapshot, sku_snapshot, size_snapshot, color_snapshot, price_snapshot, quantity, subtotal, created_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		`, orderID, line.VariantID, line.ProductName, line.ProductSlug, line.SKU, line.Size, line.Color, line.Price, line.Quantity, line.Subtotal, seed.CreatedAt); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO payments (order_id, payment_code, method, amount, status, paid_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		ON CONFLICT (order_id) DO NOTHING
	`, orderID, seed.PaymentCode, seed.PaymentMethod, total, seed.PaymentStatus, paidAtFor(seed.PaymentStatus, seed.CreatedAt), seed.CreatedAt); err != nil {
		return err
	}

	if seed.Shipment != nil {
		if _, err := tx.Exec(ctx, `
			INSERT INTO shipments (order_id, courier, service_name, tracking_number, status, shipped_at, delivered_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
			ON CONFLICT (order_id) DO NOTHING
		`, orderID, seed.Shipment.Courier, seed.Shipment.ServiceName, seed.Shipment.TrackingNumber, seed.Shipment.Status, seed.Shipment.ShippedAt, seed.Shipment.DeliveredAt, seed.CreatedAt); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO order_status_logs (order_id, old_status, new_status, note, changed_by, created_at)
		VALUES ($1, NULL, $2, 'SYSTEM', NULL, $3)
	`, orderID, seed.OrderStatus, seed.CreatedAt); err != nil {
		return err
	}

	if seed.PaymentStatus == "PAID" {
		if err := applyDemoSales(ctx, tx, lines, seed.CreatedAt, customerID); err != nil {
			return err
		}
	}
	return nil
}

type orderLineDetail struct {
	VariantID   uuid.UUID
	ProductName string
	ProductSlug string
	SKU         string
	Size        string
	Color       string
	Price       float64
	Quantity    int
	Subtotal    float64
}

func buildOrderTotals(ctx context.Context, tx pgx.Tx, lines []demoOrderLineSeed, variantIDs map[string]uuid.UUID) (float64, float64, float64, []orderLineDetail, error) {
	out := make([]orderLineDetail, 0, len(lines))
	var subtotal float64
	for _, line := range lines {
		variantID := variantIDs[line.VariantSKU]
		if variantID == uuid.Nil {
			return 0, 0, 0, nil, fmt.Errorf("variant not found for order seed: %s", line.VariantSKU)
		}
		var detail orderLineDetail
		detail.VariantID = variantID
		detail.Quantity = line.Quantity
		if err := tx.QueryRow(ctx, `
			SELECT pv.sku, pv.size, pv.color, pv.price, p.name, p.slug
			FROM product_variants pv
			JOIN products p ON p.id = pv.product_id
			WHERE pv.id=$1
		`, variantID).Scan(&detail.SKU, &detail.Size, &detail.Color, &detail.Price, &detail.ProductName, &detail.ProductSlug); err != nil {
			return 0, 0, 0, nil, err
		}
		detail.Subtotal = detail.Price * float64(detail.Quantity)
		subtotal += detail.Subtotal
		out = append(out, detail)
	}
	shipping := 0.0
	if subtotal < 500000 {
		shipping = 20000
	}
	total := subtotal + shipping
	return subtotal, shipping, total, out, nil
}

func applyDemoSales(ctx context.Context, tx pgx.Tx, lines []orderLineDetail, createdAt time.Time, userID uuid.UUID) error {
	for _, line := range lines {
		var stock int
		if err := tx.QueryRow(ctx, `SELECT stock FROM product_variants WHERE id=$1 FOR UPDATE`, line.VariantID).Scan(&stock); err != nil {
			return err
		}
		nextStock := stock - line.Quantity
		if nextStock < 0 {
			nextStock = 0
		}
		status := "ACTIVE"
		if nextStock == 0 {
			status = "OUT_OF_STOCK"
		}
		if _, err := tx.Exec(ctx, `UPDATE product_variants SET stock=$1, status=$2, updated_at=$3 WHERE id=$4`, nextStock, status, createdAt, line.VariantID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO inventory_logs (product_variant_id, type, quantity, previous_stock, current_stock, note, created_by, created_at)
			VALUES ($1, 'SALE', $2, $3, $4, 'Demo order seed', $5, $6)
		`, line.VariantID, -line.Quantity, stock, nextStock, userID, createdAt); err != nil {
			return err
		}
	}
	return nil
}

func paidAtFor(status string, createdAt time.Time) *time.Time {
	if status != "PAID" {
		return nil
	}
	t := createdAt.Add(30 * time.Minute)
	return &t
}

func ensureDemoAssets(assetsDir string) error {
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		return err
	}
	assets := map[string]string{
		"aeroflex-runner-1.svg": demoSVG("AeroFlex", "#0f172a", "#38bdf8"),
		"aeroflex-runner-2.svg": demoSVG("Runner", "#1e293b", "#22c55e"),
		"urban-glide-1.svg":     demoSVG("Urban", "#111827", "#f97316"),
		"urban-glide-2.svg":     demoSVG("Glide", "#334155", "#eab308"),
		"core-trainer-1.svg":    demoSVG("Trainer", "#1f2937", "#a78bfa"),
		"core-trainer-2.svg":    demoSVG("Core", "#0f172a", "#f472b6"),
		"strike-pro-tf-1.svg":   demoSVG("Strike", "#111827", "#ef4444"),
		"strike-pro-tf-2.svg":   demoSVG("Pro TF", "#374151", "#facc15"),
		"coast-slide-1.svg":     demoSVG("Coast", "#0f172a", "#34d399"),
		"coast-slide-2.svg":     demoSVG("Slide", "#1e293b", "#fb7185"),
	}
	for name, content := range assets {
		path := filepath.Join(assetsDir, name)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func demoSVG(title, dark, accent string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 800" role="img" aria-label="%[1]s">
  <defs>
    <linearGradient id="bg" x1="0%%" y1="0%%" x2="100%%" y2="100%%">
      <stop offset="0%%" stop-color="%[2]s"/>
      <stop offset="100%%" stop-color="%[3]s"/>
    </linearGradient>
  </defs>
  <rect width="800" height="800" rx="72" fill="url(#bg)"/>
  <circle cx="620" cy="180" r="120" fill="rgba(255,255,255,0.10)"/>
  <circle cx="180" cy="620" r="160" fill="rgba(255,255,255,0.08)"/>
  <text x="80" y="430" fill="#ffffff" font-family="Arial, sans-serif" font-size="88" font-weight="700">%[1]s</text>
  <text x="80" y="520" fill="rgba(255,255,255,0.85)" font-family="Arial, sans-serif" font-size="36">Demo product image</text>
</svg>`, title, dark, accent)
}

func mustDemoPasswordHash(password string) string {
	hash, err := security.HashPassword(password)
	if err != nil {
		panic(err)
	}
	return hash
}

func lookupUUID(ctx context.Context, tx pgx.Tx, query string, arg any) (uuid.UUID, error) {
	var id uuid.UUID
	if err := tx.QueryRow(ctx, query, arg).Scan(&id); err != nil {
		return uuid.UUID{}, err
	}
	return id, nil
}

func ptrString(v string) *string {
	return &v
}

func ptrTime(v time.Time) *time.Time {
	return &v
}

func optionalString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
