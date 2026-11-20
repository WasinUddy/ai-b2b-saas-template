package handler

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	fiberrecover "github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/wasinuddy/ai-b2b-saas-template/internal/middleware"
)

// Dependencies holds the handlers and runtime options mounted by Register.
type Dependencies struct {
	Health    *HealthHandler
	AuthGuard fiber.Handler
	Log       *slog.Logger
	WebUIDir  string
}

// Register mounts middleware and routes on app.
// recover runs inside Logger so a panic becomes a logged 500.
// A non-empty WebUIDir is served as the single-page frontend after every API
// route, so it never shadows /healthz, /readyz, or /api.
func Register(app *fiber.App, deps Dependencies) {
	app.Use(middleware.Logger(deps.Log))
	app.Use(fiberrecover.New())
	app.Get("/healthz", deps.Health.Live)
	app.Get("/readyz", deps.Health.Ready)

	// Mount protected API routes with deps.AuthGuard when adding endpoints, e.g.:
	//   v1 := app.Group("/api/v1")
	//   protected := v1.Group("", deps.AuthGuard)
	//   protected.Get("/items", itemHandler.List)
	if deps.AuthGuard != nil {
		app.Group("/api/v1", deps.AuthGuard)
	}

	app.Use("/api", notFound)
	if deps.WebUIDir != "" {
		registerWebUI(app, deps.WebUIDir)
	}
	app.Use(notFound)
}

func notFound(_ *fiber.Ctx) error {
	return fiber.NewError(fiber.StatusNotFound, "not found")
}
