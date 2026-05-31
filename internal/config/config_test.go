package config

import (
	"os"
	"testing"
	"time"
)

// clearEnv unsets all config-related env vars so tests start clean.
func clearEnv(t *testing.T) {
	t.Helper()
	vars := []string{
		"API_PORT", "API_READ_TIMEOUT", "API_WRITE_TIMEOUT", "API_IDLE_TIMEOUT",
		"DATABASE_URL", "PG_MAX_OPEN_CONNS", "PG_MAX_IDLE_CONNS", "PG_CONN_MAX_LIFETIME",
		"REDIS_URL", "REDIS_PASSWORD", "REDIS_DB",
		"WORKER_CONCURRENCY", "QUEUE_RETRY_MAX", "QUEUE_RETRY_DELAY",
		"LOG_LEVEL", "LOG_FORMAT", "LOG_ADD_SOURCE",
		"SERVICE_NAME", "ENVIRONMENT",
	}
	for _, v := range vars {
		os.Unsetenv(v)
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// API defaults
	if cfg.API.Port != 8080 {
		t.Errorf("API.Port = %d, want 8080", cfg.API.Port)
	}
	if cfg.API.ReadTimeout != 10*time.Second {
		t.Errorf("API.ReadTimeout = %v, want 10s", cfg.API.ReadTimeout)
	}
	if cfg.API.WriteTimeout != 30*time.Second {
		t.Errorf("API.WriteTimeout = %v, want 30s", cfg.API.WriteTimeout)
	}
	if cfg.API.IdleTimeout != 60*time.Second {
		t.Errorf("API.IdleTimeout = %v, want 60s", cfg.API.IdleTimeout)
	}

	// Postgres defaults
	if cfg.Postgres.URL == "" {
		t.Error("Postgres.URL should have default value")
	}
	if cfg.Postgres.MaxOpenConns != 25 {
		t.Errorf("Postgres.MaxOpenConns = %d, want 25", cfg.Postgres.MaxOpenConns)
	}
	if cfg.Postgres.MaxIdleConns != 5 {
		t.Errorf("Postgres.MaxIdleConns = %d, want 5", cfg.Postgres.MaxIdleConns)
	}
	if cfg.Postgres.ConnMaxLifetime != 5*time.Minute {
		t.Errorf("Postgres.ConnMaxLifetime = %v, want 5m", cfg.Postgres.ConnMaxLifetime)
	}

	// Redis defaults
	if cfg.Redis.URL == "" {
		t.Error("Redis.URL should have default value")
	}
	if cfg.Redis.Password != "" {
		t.Errorf("Redis.Password = %q, want empty", cfg.Redis.Password)
	}
	if cfg.Redis.DB != 0 {
		t.Errorf("Redis.DB = %d, want 0", cfg.Redis.DB)
	}

	// Queue defaults
	if cfg.Queue.Concurrency != 10 {
		t.Errorf("Queue.Concurrency = %d, want 10", cfg.Queue.Concurrency)
	}
	if cfg.Queue.RetryMaxAttempts != 3 {
		t.Errorf("Queue.RetryMaxAttempts = %d, want 3", cfg.Queue.RetryMaxAttempts)
	}
	if cfg.Queue.RetryDelay != 30*time.Second {
		t.Errorf("Queue.RetryDelay = %v, want 30s", cfg.Queue.RetryDelay)
	}

	// Logging defaults
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %s, want info", cfg.Logging.Level)
	}
	if cfg.Logging.AddSource != false {
		t.Errorf("Logging.AddSource = %v, want false", cfg.Logging.AddSource)
	}

	// Application defaults
	if cfg.ServiceName != "posthub" {
		t.Errorf("ServiceName = %s, want posthub", cfg.ServiceName)
	}
	if cfg.Environment != "development" {
		t.Errorf("Environment = %s, want development", cfg.Environment)
	}
}

func TestLoad_LogFormatDerivedFromEnvironment(t *testing.T) {
	clearEnv(t)

	t.Setenv("ENVIRONMENT", "production")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Logging.Format = %s, want json for production", cfg.Logging.Format)
	}
}

func TestLoad_LogFormatExplicitOverride(t *testing.T) {
	clearEnv(t)

	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("LOG_FORMAT", "console")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Logging.Format != "console" {
		t.Errorf("Logging.Format = %s, want console (explicit override)", cfg.Logging.Format)
	}
}

func TestLoad_Overrides(t *testing.T) {
	clearEnv(t)

	t.Setenv("API_PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("WORKER_CONCURRENCY", "5")
	t.Setenv("PG_MAX_OPEN_CONNS", "50")
	t.Setenv("REDIS_DB", "3")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.API.Port != 9090 {
		t.Errorf("API.Port = %d, want 9090", cfg.API.Port)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %s, want debug", cfg.Logging.Level)
	}
	if cfg.Queue.Concurrency != 5 {
		t.Errorf("Queue.Concurrency = %d, want 5", cfg.Queue.Concurrency)
	}
	if cfg.Postgres.MaxOpenConns != 50 {
		t.Errorf("Postgres.MaxOpenConns = %d, want 50", cfg.Postgres.MaxOpenConns)
	}
	if cfg.Redis.DB != 3 {
		t.Errorf("Redis.DB = %d, want 3", cfg.Redis.DB)
	}
}

func TestLoad_DurationOverrides(t *testing.T) {
	clearEnv(t)

	t.Setenv("API_READ_TIMEOUT", "5s")
	t.Setenv("PG_CONN_MAX_LIFETIME", "10m")
	t.Setenv("QUEUE_RETRY_DELAY", "1m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.API.ReadTimeout != 5*time.Second {
		t.Errorf("API.ReadTimeout = %v, want 5s", cfg.API.ReadTimeout)
	}
	if cfg.Postgres.ConnMaxLifetime != 10*time.Minute {
		t.Errorf("Postgres.ConnMaxLifetime = %v, want 10m", cfg.Postgres.ConnMaxLifetime)
	}
	if cfg.Queue.RetryDelay != 1*time.Minute {
		t.Errorf("Queue.RetryDelay = %v, want 1m", cfg.Queue.RetryDelay)
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := &Config{
		API:      APIConfig{Port: 0},
		Postgres: PostgresConfig{URL: "postgres://localhost/db", MaxOpenConns: 1, MaxIdleConns: 1},
		Redis:    RedisConfig{URL: "redis://localhost"},
		Queue:    QueueConfig{Concurrency: 1},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestValidate_InvalidConcurrency(t *testing.T) {
	cfg := &Config{
		API:      APIConfig{Port: 8080},
		Postgres: PostgresConfig{URL: "postgres://localhost/db", MaxOpenConns: 1, MaxIdleConns: 1},
		Redis:    RedisConfig{URL: "redis://localhost"},
		Queue:    QueueConfig{Concurrency: 0},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid concurrency")
	}
}

func TestValidate_MissingDatabaseURL(t *testing.T) {
	cfg := &Config{
		API:      APIConfig{Port: 8080},
		Postgres: PostgresConfig{URL: "", MaxOpenConns: 1, MaxIdleConns: 1},
		Redis:    RedisConfig{URL: "redis://localhost"},
		Queue:    QueueConfig{Concurrency: 1},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for missing DATABASE_URL")
	}
}

func TestValidate_MissingRedisURL(t *testing.T) {
	cfg := &Config{
		API:      APIConfig{Port: 8080},
		Postgres: PostgresConfig{URL: "postgres://localhost/db", MaxOpenConns: 1, MaxIdleConns: 1},
		Redis:    RedisConfig{URL: ""},
		Queue:    QueueConfig{Concurrency: 1},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for missing REDIS_URL")
	}
}

func TestValidate_InvalidRedisDB(t *testing.T) {
	cfg := &Config{
		API:      APIConfig{Port: 8080},
		Postgres: PostgresConfig{URL: "postgres://localhost/db", MaxOpenConns: 1, MaxIdleConns: 1},
		Redis:    RedisConfig{URL: "redis://localhost", DB: 16},
		Queue:    QueueConfig{Concurrency: 1},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for REDIS_DB > 15")
	}
}

func TestValidate_InvalidMaxOpenConns(t *testing.T) {
	cfg := &Config{
		API:      APIConfig{Port: 8080},
		Postgres: PostgresConfig{URL: "postgres://localhost/db", MaxOpenConns: 0, MaxIdleConns: 1},
		Redis:    RedisConfig{URL: "redis://localhost"},
		Queue:    QueueConfig{Concurrency: 1},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for PG_MAX_OPEN_CONNS < 1")
	}
}

func TestValidate_InvalidMaxIdleConns(t *testing.T) {
	cfg := &Config{
		API:      APIConfig{Port: 8080},
		Postgres: PostgresConfig{URL: "postgres://localhost/db", MaxOpenConns: 1, MaxIdleConns: 0},
		Redis:    RedisConfig{URL: "redis://localhost"},
		Queue:    QueueConfig{Concurrency: 1},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for PG_MAX_IDLE_CONNS < 1")
	}
}

func TestEnvStr_Present(t *testing.T) {
	t.Setenv("TEST_ENV_STR", "hello")
	if got := envStr("TEST_ENV_STR", "default"); got != "hello" {
		t.Errorf("envStr() = %q, want hello", got)
	}
}

func TestEnvStr_Missing(t *testing.T) {
	os.Unsetenv("TEST_ENV_STR_MISSING")
	if got := envStr("TEST_ENV_STR_MISSING", "default"); got != "default" {
		t.Errorf("envStr() = %q, want default", got)
	}
}

func TestEnvInt_Present(t *testing.T) {
	t.Setenv("TEST_ENV_INT", "42")
	if got := envInt("TEST_ENV_INT", 0); got != 42 {
		t.Errorf("envInt() = %d, want 42", got)
	}
}

func TestEnvInt_Invalid(t *testing.T) {
	t.Setenv("TEST_ENV_INT_BAD", "notanumber")
	if got := envInt("TEST_ENV_INT_BAD", 99); got != 99 {
		t.Errorf("envInt() = %d, want 99 (default for invalid input)", got)
	}
}

func TestEnvBool_Present(t *testing.T) {
	t.Setenv("TEST_ENV_BOOL", "true")
	if got := envBool("TEST_ENV_BOOL", false); got != true {
		t.Errorf("envBool() = %v, want true", got)
	}
}

func TestEnvBool_Invalid(t *testing.T) {
	t.Setenv("TEST_ENV_BOOL_BAD", "maybe")
	if got := envBool("TEST_ENV_BOOL_BAD", false); got != false {
		t.Errorf("envBool() = %v, want false (default for invalid input)", got)
	}
}

func TestEnvDuration_Present(t *testing.T) {
	t.Setenv("TEST_ENV_DUR", "5s")
	if got := envDuration("TEST_ENV_DUR", time.Second); got != 5*time.Second {
		t.Errorf("envDuration() = %v, want 5s", got)
	}
}

func TestEnvDuration_Invalid(t *testing.T) {
	t.Setenv("TEST_ENV_DUR_BAD", "notaduration")
	if got := envDuration("TEST_ENV_DUR_BAD", time.Second); got != time.Second {
		t.Errorf("envDuration() = %v, want 1s (default for invalid input)", got)
	}
}
