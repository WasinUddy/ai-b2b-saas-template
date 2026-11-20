package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

const (
	defaultHTTPAddr      = ":8080"
	defaultWebUIDir      = "webui/dist"
	defaultDBDSN         = "file:app.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	defaultDBAutoMigrate = true
	defaultAuthEnabled   = false
)

// Config is the process configuration for the API.
type Config struct {
	HTTPAddr      string
	DBDSN         string
	DBAutoMigrate bool
	// WebUIDir is the built frontend directory. Empty disables frontend serving.
	WebUIDir string
	// AuthEnabled toggles middleware.RequireAuth enforcement (disabled by default).
	AuthEnabled bool
}

// Load reads configuration from the environment and applies defaults.
// An empty HTTP_ADDR or DB_DSN is rejected. An empty WEBUI_DIR disables
// frontend serving.
func Load() (Config, error) {
	autoMigrate, err := envBool("DB_AUTO_MIGRATE", defaultDBAutoMigrate)
	if err != nil {
		return Config{}, err
	}
	authEnabled, err := envBool("AUTH_ENABLED", defaultAuthEnabled)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPAddr:      env("HTTP_ADDR", defaultHTTPAddr),
		DBDSN:         env("DB_DSN", defaultDBDSN),
		DBAutoMigrate: autoMigrate,
		WebUIDir:      env("WEBUI_DIR", defaultWebUIDir),
		AuthEnabled:   authEnabled,
	}
	if cfg.HTTPAddr == "" {
		return Config{}, errors.New("empty HTTP_ADDR")
	}
	if cfg.DBDSN == "" {
		return Config{}, errors.New("empty DB_DSN")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) (bool, error) {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}
