// Package main is the composition root for the HTTP API.
//
// It wires configuration, SQLite, migrations, repositories, services, and
// routes, then serves until the process is signaled to stop.
//
//	@title				AI B2B SaaS API
//	@version			1.0
//	@description		HTTP API for the AI B2B SaaS template.
//	@BasePath			/
//	@tag.name			health
//	@tag.description	Process liveness and readiness.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/wasinuddy/ai-b2b-saas-template/internal/handler"
	"github.com/wasinuddy/ai-b2b-saas-template/internal/middleware"
	"github.com/wasinuddy/ai-b2b-saas-template/internal/platform/config"
	"github.com/wasinuddy/ai-b2b-saas-template/internal/platform/database"
	"github.com/wasinuddy/ai-b2b-saas-template/internal/platform/gormrepo"
	"github.com/wasinuddy/ai-b2b-saas-template/internal/service"
	"github.com/wasinuddy/ai-b2b-saas-template/migrations"
)

const (
	migrateTimeout  = 30 * time.Second
	shutdownTimeout = 10 * time.Second
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("startup failed", slog.String("err", err.Error()))
		os.Exit(1)
	}
}

// webUIDir returns dir when it holds a built frontend, otherwise "" so the API
// runs without one.
func webUIDir(dir string, log *slog.Logger) string {
	if dir == "" {
		return ""
	}
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		log.Warn("frontend not served", slog.String("dir", dir), slog.String("err", err.Error()))
		return ""
	}
	log.Info("serving frontend", slog.String("dir", dir))
	return dir
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := database.Open(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := database.Close(db); closeErr != nil {
			log.Error("close database", slog.String("err", closeErr.Error()))
		}
	}()

	if cfg.DBAutoMigrate {
		migCtx, migCancel := context.WithTimeout(context.Background(), migrateTimeout)
		applied, migErr := database.Migrate(migCtx, db, migrations.Files)
		migCancel()
		if migErr != nil {
			return fmt.Errorf("auto-migrate: %w", migErr)
		}
		log.Info("database migrated", slog.Int("applied", applied))
	}

	healthService := service.NewHealthService(gormrepo.NewHealthRepository(db))

	// Auth middleware is disabled by default (AUTH_ENABLED=false).
	// Pass a middleware.TokenVerifier implementation here when enabling auth.
	var tokenVerifier middleware.TokenVerifier
	if cfg.AuthEnabled {
		log.Info("authentication middleware enabled")
	}

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          handler.NewErrorHandler(log),
	})
	handler.Register(app, handler.Dependencies{
		Health:    handler.NewHealthHandler(healthService),
		AuthGuard: middleware.RequireAuth(tokenVerifier, cfg.AuthEnabled),
		Log:       log,
		WebUIDir:  webUIDir(cfg.WebUIDir, log),
	})

	return serveUntilSignal(app, cfg.HTTPAddr, log)
}

func serveUntilSignal(app *fiber.App, addr string, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", slog.String("addr", addr))
		if listenErr := app.Listen(addr); listenErr != nil {
			errCh <- listenErr
		}
	}()

	select {
	case listenErr := <-errCh:
		return fmt.Errorf("listen: %w", listenErr)
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if shutdownErr := app.ShutdownWithContext(shutdownCtx); shutdownErr != nil {
		return fmt.Errorf("shutdown: %w", shutdownErr)
	}
	return nil
}
