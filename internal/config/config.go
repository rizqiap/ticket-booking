package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds application settings.
type Config struct {
	AppPort           int
	DBUrl             string
	JWTSecret         string
	MidtransServerKey string
}

// Load reads and validates environment settings.
func Load() (*Config, error) {
	// load .env if avail
	_ = godotenv.Load()

	// load APP_PORT
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	// Convert string to int
	appPort, err := strconv.Atoi(port)
	if err != nil {
		return nil, fmt.Errorf("invalid APP_PORT: %w", err)
	}

	// Validate port range
	if appPort < 1 || appPort > 65535 {
		return nil, fmt.Errorf("APP_PORT must be between 1 and 65535")
	}

	// Read environment variables
	databaseURL := os.Getenv("DATABASE_URL")
	jwtSecret := os.Getenv("JWT_SECRET")
	midtransServerKey := strings.TrimSpace(os.Getenv("MIDTRANS_SERVER_KEY"))

	// Validate required variables
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if midtransServerKey == "" {
		return nil, fmt.Errorf("MIDTRANS_SERVER_KEY is required")
	}

	// Initialize configuration
	cfg := &Config{
		AppPort:           appPort,
		DBUrl:             databaseURL,
		JWTSecret:         jwtSecret,
		MidtransServerKey: midtransServerKey,
	}

	return cfg, nil
}
