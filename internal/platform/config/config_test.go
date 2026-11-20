package config_test

import (
	"testing"

	"github.com/wasinuddy/ai-b2b-saas-template/internal/platform/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("DB_DSN", "file:test.db")
	t.Setenv("DB_AUTO_MIGRATE", "")
	t.Setenv("AUTH_ENABLED", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.DBAutoMigrate {
		t.Fatal("DBAutoMigrate = false, want true")
	}
	if cfg.AuthEnabled {
		t.Fatal("AuthEnabled = true, want false")
	}
}

func TestLoadParsesBooleans(t *testing.T) {
	t.Setenv("DB_AUTO_MIGRATE", "false")
	t.Setenv("AUTH_ENABLED", "true")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DBAutoMigrate {
		t.Fatal("DBAutoMigrate = true, want false")
	}
	if !cfg.AuthEnabled {
		t.Fatal("AuthEnabled = false, want true")
	}
}

func TestLoadRejectsInvalidBoolean(t *testing.T) {
	t.Setenv("AUTH_ENABLED", "not-a-bool")

	if _, err := config.Load(); err == nil {
		t.Fatal("expected error for invalid AUTH_ENABLED, got nil")
	}
}
