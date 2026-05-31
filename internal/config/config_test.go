package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear env vars to test defaults
	os.Unsetenv("API_PORT")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("REDIS_URL")
	os.Unsetenv("WORKER_CONCURRENCY")
	os.Unsetenv("LOG_LEVEL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.APIPort != 8080 {
		t.Errorf("APIPort = %d, want 8080", cfg.APIPort)
	}
	if cfg.DatabaseURL == "" {
		t.Error("DatabaseURL should have default value")
	}
	if cfg.RedisURL == "" {
		t.Error("RedisURL should have default value")
	}
	if cfg.WorkerConcurrency != 10 {
		t.Errorf("WorkerConcurrency = %d, want 10", cfg.WorkerConcurrency)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %s, want info", cfg.LogLevel)
	}
	if cfg.ServiceName != "posthub" {
		t.Errorf("ServiceName = %s, want posthub", cfg.ServiceName)
	}
}

func TestLoad_Overrides(t *testing.T) {
	t.Setenv("API_PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("WORKER_CONCURRENCY", "5")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.APIPort != 9090 {
		t.Errorf("APIPort = %d, want 9090", cfg.APIPort)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %s, want debug", cfg.LogLevel)
	}
	if cfg.WorkerConcurrency != 5 {
		t.Errorf("WorkerConcurrency = %d, want 5", cfg.WorkerConcurrency)
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := &Config{
		APIPort:           0,
		DatabaseURL:       "postgres://localhost/db",
		RedisURL:          "redis://localhost",
		WorkerConcurrency: 1,
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestValidate_InvalidConcurrency(t *testing.T) {
	cfg := &Config{
		APIPort:           8080,
		DatabaseURL:       "postgres://localhost/db",
		RedisURL:          "redis://localhost",
		WorkerConcurrency: 0,
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid concurrency")
	}
}
