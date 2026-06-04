package config

import (
	"bufio"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv          string
	AppPort         string
	AppBaseURL      string
	FrontendURL     string
	DBHost          string
	DBPort          string
	DBUser          string
	DBPassword      string
	DBName          string
	DBSSLMode       string
	JWTSecret       string
	JWTExpiresIn    time.Duration
	UploadDir       string
	MaxUploadSizeMB int64
	MaxBodySizeMB   int64
	ResetDemoData   bool
	LogLevel        slog.Level
}

func Load() Config {
	_ = loadDotEnv(".env")
	return Config{
		AppEnv:          getEnv("APP_ENV", "development"),
		AppPort:         getEnv("APP_PORT", "8080"),
		AppBaseURL:      getEnv("APP_BASE_URL", "http://localhost:8080"),
		FrontendURL:     getEnv("FRONTEND_URL", "http://localhost:3000"),
		DBHost:          getEnv("DB_HOST", "localhost"),
		DBPort:          getEnv("DB_PORT", "5432"),
		DBUser:          getEnv("DB_USER", "postgres"),
		DBPassword:      getEnv("DB_PASSWORD", "postgres"),
		DBName:          getEnv("DB_NAME", "footwear_commerce"),
		DBSSLMode:       getEnv("DB_SSLMODE", "disable"),
		JWTSecret:       getEnv("JWT_SECRET", "change-me"),
		JWTExpiresIn:    mustDuration(getEnv("JWT_EXPIRES_IN", "24h")),
		UploadDir:       getEnv("UPLOAD_DIR", "./uploads"),
		MaxUploadSizeMB: mustInt64(getEnv("MAX_UPLOAD_SIZE_MB", "2")),
		MaxBodySizeMB:   mustInt64(getEnv("MAX_BODY_SIZE_MB", "2")),
		ResetDemoData:   mustBool(getEnv("RESET_DEMO_DATA", "false")),
		LogLevel:        parseLogLevel(getEnv("LOG_LEVEL", "info")),
	}
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
	return scanner.Err()
}

func (c Config) DatabaseURL() string {
	return "postgres://" + c.DBUser + ":" + c.DBPassword + "@" + c.DBHost + ":" + c.DBPort + "/" + c.DBName + "?sslmode=" + c.DBSSLMode
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func mustDuration(v string) time.Duration {
	d, err := time.ParseDuration(v)
	if err != nil {
		return 24 * time.Hour
	}
	return d
}

func mustInt64(v string) int64 {
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 2
	}
	return n
}

func mustBool(v string) bool {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return b
}

func parseLogLevel(v string) slog.Level {
	switch strings.ToLower(v) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
