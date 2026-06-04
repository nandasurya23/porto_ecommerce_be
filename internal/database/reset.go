package database

import (
	"context"
	"fmt"
	"strings"
)

// ResetDemoData clears all mutable application tables while preserving schema_migrations.
// It is intended for local development only.
func (db *DB) ResetDemoData(ctx context.Context) error {
	tables := []string{
		"order_status_logs",
		"inventory_logs",
		"shipments",
		"payments",
		"order_items",
		"orders",
		"cart_items",
		"carts",
		"product_images",
		"product_variants",
		"products",
		"addresses",
		"categories",
		"users",
	}
	stmt := "TRUNCATE TABLE " + strings.Join(tables, ", ") + " RESTART IDENTITY CASCADE"
	if _, err := db.Pool.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("reset demo data: %w", err)
	}
	return nil
}
