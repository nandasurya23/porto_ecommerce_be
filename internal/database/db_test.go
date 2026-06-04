package database

import (
	"strings"
	"testing"

	"footwear-backend/internal/config"
)

func TestResolveDatabaseURLUsesRawURLWhenPresent(t *testing.T) {
	cfg := config.Config{
		AppEnv:         "production",
		DatabaseURLRaw: "postgresql://user:pass@host:5432/db?sslmode=require",
		DBHost:         "localhost",
		DBPort:         "5432",
		DBUser:         "postgres",
		DBPassword:     "postgres",
		DBName:         "footwear_commerce",
		DBSSLMode:      "disable",
	}

	got, err := resolveDatabaseURL(cfg)
	if err != nil {
		t.Fatalf("resolveDatabaseURL returned error: %v", err)
	}
	if got != cfg.DatabaseURLRaw {
		t.Fatalf("expected raw DATABASE_URL, got %q", got)
	}
}

func TestResolveDatabaseURLRejectsProductionWithoutRawURL(t *testing.T) {
	cfg := config.Config{
		AppEnv:     "production",
		DBHost:     "localhost",
		DBPort:     "5432",
		DBUser:     "postgres",
		DBPassword: "postgres",
		DBName:     "footwear_commerce",
		DBSSLMode:  "disable",
	}

	got, err := resolveDatabaseURL(cfg)
	if err == nil {
		t.Fatalf("expected error, got DSN %q", got)
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected DATABASE_URL error, got %v", err)
	}
}

func TestResolveDatabaseURLBuildsLocalDevFallback(t *testing.T) {
	cfg := config.Config{
		AppEnv:     "development",
		DBHost:     "localhost",
		DBPort:     "5433",
		DBUser:     "postgres",
		DBPassword: "postgres",
		DBName:     "footwear_commerce",
		DBSSLMode:  "disable",
	}

	got, err := resolveDatabaseURL(cfg)
	if err != nil {
		t.Fatalf("resolveDatabaseURL returned error: %v", err)
	}
	want := "postgres://postgres:postgres@localhost:5433/footwear_commerce?sslmode=disable"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
