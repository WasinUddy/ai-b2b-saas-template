package service

import (
	"context"
	"fmt"

	"github.com/wasinuddy/ai-b2b-saas-template/internal/model"
	"github.com/wasinuddy/ai-b2b-saas-template/internal/repository"
)

// HealthService reports whether the process can serve traffic.
type HealthService struct {
	health repository.HealthRepository
}

// NewHealthService returns a health use case that checks health.
func NewHealthService(health repository.HealthRepository) *HealthService {
	return &HealthService{health: health}
}

// Ready reports that dependencies needed to serve traffic are reachable.
func (s *HealthService) Ready(ctx context.Context) (model.HealthStatus, error) {
	if err := s.health.Ping(ctx); err != nil {
		return model.HealthStatus{}, fmt.Errorf("readiness: %w", err)
	}
	return model.HealthStatus{Status: "ok"}, nil
}
