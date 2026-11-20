package handler

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"github.com/wasinuddy/ai-b2b-saas-template/internal/model"
	"github.com/wasinuddy/ai-b2b-saas-template/internal/service"
)

// HealthResponse is the liveness or readiness payload returned to clients.
type HealthResponse struct {
	Status string `json:"status"`
}

type healthChecker interface {
	Ready(ctx context.Context) (model.HealthStatus, error)
}

// HealthHandler serves liveness and readiness routes.
type HealthHandler struct {
	health healthChecker
}

// NewHealthHandler returns the HTTP adapter for health checks.
func NewHealthHandler(health *service.HealthService) *HealthHandler {
	return &HealthHandler{health: health}
}

// Live reports that the process is running.
//
//	@Summary		Liveness
//	@Description	Reports that the process is running.
//	@Tags			health
//	@ID				getHealth
//	@Produce		json
//	@Success		200	{object}	HealthResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/healthz [get]
func (h *HealthHandler) Live(c *fiber.Ctx) error {
	return respond(c, fiber.StatusOK, HealthResponse{Status: "ok"})
}

// Ready reports whether the process can serve traffic.
//
//	@Summary		Readiness
//	@Description	Reports whether the process can serve traffic.
//	@Tags			health
//	@ID				getReady
//	@Produce		json
//	@Success		200	{object}	HealthResponse
//	@Failure		503	{object}	ErrorResponse
//	@Router			/readyz [get]
func (h *HealthHandler) Ready(c *fiber.Ctx) error {
	status, err := h.health.Ready(c.UserContext())
	if err != nil {
		return newAPIError(fiber.StatusServiceUnavailable, "not ready", err)
	}
	return respond(c, fiber.StatusOK, HealthResponse{Status: status.Status})
}
