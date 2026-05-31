// Package config provides environment-based configuration for posthub services.
//
// Configuration is loaded from environment variables with sensible defaults.
// No files, no flags — just env vars (12-factor app).
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the application configuration.
type Config struct {
	// Server
	APIPort int

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// Worker
	WorkerConcurrency int

	// Logging
	LogLevel string

	// Application
	ServiceName string
	Environment string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		APIPort:           envInt("API_PORT", 8080),
		DatabaseURL:       envStr("DATABASE_URL", "postgres://localhost:5432/posthub?sslmode=disable"),
		RedisURL:          envStr("REDIS_URL", "redis://localhost:6379"),
		WorkerConcurrency: envInt("WORKER_CONCURRENCY", 10),
		LogLevel:          envStr("LOG_LEVEL", "info"),
		ServiceName:       envStr("SERVICE_NAME", "posthub"),
		Environment:       envStr("ENVIRONMENT", "development"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that required configuration values are present.
func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.RedisURL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}
	if c.APIPort < 1 || c.APIPort > 65535 {
		return fmt.Errorf("API_PORT must be between 1 and 65535, got %d", c.APIPort)
	}
	if c.WorkerConcurrency < 1 {
		return fmt.Errorf("WORKER_CONCURRENCY must be at least 1, got %d", c.WorkerConcurrency)
	}
	return nil
}

func envStr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}
