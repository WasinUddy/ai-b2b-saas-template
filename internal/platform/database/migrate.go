package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"gorm.io/gorm"
)

const createMigrationsTableSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version TEXT PRIMARY KEY,
	applied_at TEXT NOT NULL
);`

// Migrate applies every pending *.sql file from files in lexicographical order.
// It returns the number of migrations applied during this call.
func Migrate(ctx context.Context, db *gorm.DB, files fs.FS) (int, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return 0, fmt.Errorf("sqlite pool: %w", err)
	}
	if _, execErr := sqlDB.ExecContext(ctx, createMigrationsTableSQL); execErr != nil {
		return 0, fmt.Errorf("ensure schema_migrations: %w", execErr)
	}

	applied, err := loadAppliedMigrations(ctx, sqlDB)
	if err != nil {
		return 0, err
	}

	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return 0, fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(names)

	count := 0
	for _, name := range names {
		if applied[name] {
			continue
		}
		if applyErr := applyMigrationFile(ctx, sqlDB, files, name); applyErr != nil {
			return count, applyErr
		}
		count++
	}
	return count, nil
}

func loadAppliedMigrations(ctx context.Context, sqlDB *sql.DB) (applied map[string]bool, retErr error) {
	rows, err := sqlDB.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close migration rows: %w", closeErr))
		}
	}()

	applied = make(map[string]bool)
	for rows.Next() {
		var version string
		if scanErr := rows.Scan(&version); scanErr != nil {
			return nil, fmt.Errorf("scan migration version: %w", scanErr)
		}
		applied[version] = true
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate schema_migrations: %w", rowsErr)
	}
	return applied, nil
}

func applyMigrationFile(ctx context.Context, sqlDB *sql.DB, files fs.FS, name string) error {
	raw, err := fs.ReadFile(files, name)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", name, err)
	}

	if _, execErr := tx.ExecContext(ctx, string(raw)); execErr != nil {
		return rollbackWith(tx, fmt.Errorf("exec migration %s: %w", name, execErr))
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, recErr := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, name, now); recErr != nil {
		return rollbackWith(tx, fmt.Errorf("record migration %s: %w", name, recErr))
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("commit migration %s: %w", name, commitErr)
	}
	return nil
}

func rollbackWith(tx *sql.Tx, cause error) error {
	if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
		return errors.Join(cause, fmt.Errorf("rollback: %w", rbErr))
	}
	return cause
}
