package database

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/wasinuddy/ai-b2b-saas-template/internal/platform/config"
)

// Open opens the SQLite database described by cfg.
func Open(cfg config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(cfg.DBDSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	return db, nil
}

// Close releases the underlying SQLite connection pool.
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("sqlite pool: %w", err)
	}
	if closeErr := sqlDB.Close(); closeErr != nil {
		return fmt.Errorf("close sqlite: %w", closeErr)
	}
	return nil
}
