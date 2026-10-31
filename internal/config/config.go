package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort   int
	DBUrl     string
	JWTSecret string
}

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

	// Validate required variables
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	// Initialize configuration
	cfg := &Config{
		AppPort:   appPort,
		DBUrl: databaseURL,
		JWTSecret: jwtSecret,
	}

	return cfg, nil
}
