// Package config provides environment-based configuration for posthub services.
//
// Configuration is loaded from environment variables with sensible defaults.
// No files, no flags — just env vars (12-factor app).
//
// All configuration is organized into sub-structs by concern: API, PostgreSQL,
// Redis, Queue, and Logging. The top-level Config composes these and adds
// application-wide fields (ServiceName, Environment).
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds the full application configuration.
type Config struct {
	API      APIConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	Queue    QueueConfig
	Logging  LoggingConfig

	// Application
	ServiceName string
	Environment string
}

// APIConfig holds HTTP server settings.
type APIConfig struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// PostgresConfig holds PostgreSQL connection settings.
type PostgresConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	URL      string
	Password string
	DB       int
}

// QueueConfig holds Asynq/worker queue settings.
type QueueConfig struct {
	Concurrency     int
	RetryMaxAttempts int
	RetryDelay      time.Duration
}

// LoggingConfig holds structured logging settings.
type LoggingConfig struct {
	Level     string
	Format    string // "json" or "console"
	AddSource bool
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		API: APIConfig{
			Port:         envInt("API_PORT", 8080),
			ReadTimeout:  envDuration("API_READ_TIMEOUT", 10*time.Second),
			WriteTimeout: envDuration("API_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:  envDuration("API_IDLE_TIMEOUT", 60*time.Second),
		},
		Postgres: PostgresConfig{
			URL:             envStr("DATABASE_URL", "postgres://localhost:5432/posthub?sslmode=disable"),
			MaxOpenConns:    envInt("PG_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    envInt("PG_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: envDuration("PG_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			URL:      envStr("REDIS_URL", "redis://localhost:6379"),
			Password: envStr("REDIS_PASSWORD", ""),
			DB:       envInt("REDIS_DB", 0),
		},
		Queue: QueueConfig{
			Concurrency:      envInt("WORKER_CONCURRENCY", 10),
			RetryMaxAttempts: envInt("QUEUE_RETRY_MAX", 3),
			RetryDelay:       envDuration("QUEUE_RETRY_DELAY", 30*time.Second),
		},
		Logging: LoggingConfig{
			Level:     envStr("LOG_LEVEL", "info"),
			Format:    envStr("LOG_FORMAT", ""),
			AddSource: envBool("LOG_ADD_SOURCE", false),
		},
		ServiceName: envStr("SERVICE_NAME", "posthub"),
		Environment: envStr("ENVIRONMENT", "development"),
	}

	// Derive log format from environment if not explicitly set
	if cfg.Logging.Format == "" {
		if cfg.Environment == "production" {
			cfg.Logging.Format = "json"
		} else {
			cfg.Logging.Format = "console"
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that required configuration values are present and valid.
func (c *Config) Validate() error {
	if c.Postgres.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.Redis.URL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}
	if c.API.Port < 1 || c.API.Port > 65535 {
		return fmt.Errorf("API_PORT must be between 1 and 65535, got %d", c.API.Port)
	}
	if c.Queue.Concurrency < 1 {
		return fmt.Errorf("WORKER_CONCURRENCY must be at least 1, got %d", c.Queue.Concurrency)
	}
	if c.Postgres.MaxOpenConns < 1 {
		return fmt.Errorf("PG_MAX_OPEN_CONNS must be at least 1, got %d", c.Postgres.MaxOpenConns)
	}
	if c.Postgres.MaxIdleConns < 1 {
		return fmt.Errorf("PG_MAX_IDLE_CONNS must be at least 1, got %d", c.Postgres.MaxIdleConns)
	}
	if c.Redis.DB < 0 || c.Redis.DB > 15 {
		return fmt.Errorf("REDIS_DB must be between 0 and 15, got %d", c.Redis.DB)
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

func envBool(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultVal
}

func envDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}
