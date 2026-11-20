package gormrepo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/wasinuddy/ai-b2b-saas-template/internal/repository"
)

// HealthRepository is the SQLite implementation of repository.HealthRepository.
type HealthRepository struct {
	db *gorm.DB
}

// NewHealthRepository returns a health repository backed by db.
func NewHealthRepository(db *gorm.DB) *HealthRepository {
	return &HealthRepository{db: db}
}

// Ping checks that SQLite accepts a connection.
func (r *HealthRepository) Ping(ctx context.Context) error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("sqlite pool: %w", err)
	}
	if pingErr := sqlDB.PingContext(ctx); pingErr != nil {
		return fmt.Errorf("ping sqlite: %w", pingErr)
	}
	return nil
}

var _ repository.HealthRepository = (*HealthRepository)(nil)
