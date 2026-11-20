// Package main runs the embedded SQLite migrations and exits.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/wasinuddy/ai-b2b-saas-template/internal/platform/config"
	"github.com/wasinuddy/ai-b2b-saas-template/internal/platform/database"
	"github.com/wasinuddy/ai-b2b-saas-template/migrations"
)

const migrateTimeout = 30 * time.Second

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("migration failed", slog.String("err", err.Error()))
		os.Exit(1)
	}
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

	ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
	defer cancel()

	applied, err := database.Migrate(ctx, db, migrations.Files)
	if err != nil {
		return err
	}
	log.Info("migrations complete", slog.Int("applied", applied))
	return nil
}
