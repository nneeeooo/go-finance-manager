package config_test

import (
	"finance-api/internal/config"
	"os"
	"testing"
)

func TestConfigLoad(t *testing.T) {
	// Set minimum required environment variables
	os.Setenv("MONOBANK_HISTORY_START", "2021-09-01")
	os.Setenv("MONOBANK_MAXSTATEMENTWINDOW", "2682000")
	os.Setenv("APP_JWT_SECRET", "test-secret")
	os.Setenv("POSTGRES_HOST", "localhost")

	cfg := config.Load("non-existent-file.env")
	if cfg == nil {
		t.Fatal("expected config to be loaded from environment")
	}

	if cfg.JWTSecret != "test-secret" {
		t.Fatalf("expected JWTSecret 'test-secret', got '%s'", cfg.JWTSecret)
	}

	if cfg.MonobankRateLimitSeconds != 60 {
		t.Fatalf("expected default rate limit 60, got %d", cfg.MonobankRateLimitSeconds)
	}
}
