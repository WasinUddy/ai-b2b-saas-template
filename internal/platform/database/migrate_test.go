package database_test

import (
	"context"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/wasinuddy/ai-b2b-saas-template/internal/platform/config"
	"github.com/wasinuddy/ai-b2b-saas-template/internal/platform/database"
	"github.com/wasinuddy/ai-b2b-saas-template/migrations"
)

func TestMigrateEmbeddedStartsClean(t *testing.T) {
	t.Parallel()

	dsn := "file:" + filepath.Join(t.TempDir(), "embedded.db") + "?_pragma=foreign_keys(1)"
	db, err := database.Open(config.Config{DBDSN: dsn})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(db); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	if _, migErr := database.Migrate(context.Background(), db, migrations.Files); migErr != nil {
		t.Fatalf("Migrate(migrations.Files) error = %v", migErr)
	}
}

func TestMigrateIsOrderedAndIdempotent(t *testing.T) {
	t.Parallel()

	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") + "?_pragma=foreign_keys(1)"
	db, err := database.Open(config.Config{DBDSN: dsn})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(db); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	fsys := fstest.MapFS{
		"0001_create_items.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE items (id TEXT PRIMARY KEY);`)},
		"0002_add_name.sql":     &fstest.MapFile{Data: []byte(`ALTER TABLE items ADD COLUMN name TEXT NOT NULL DEFAULT '';`)},
	}

	ctx := context.Background()
	applied, err := database.Migrate(ctx, db, fsys)
	if err != nil {
		t.Fatalf("first Migrate() error = %v", err)
	}
	if applied != 2 {
		t.Fatalf("applied = %d, want 2", applied)
	}

	appliedAgain, err := database.Migrate(ctx, db, fsys)
	if err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}
	if appliedAgain != 0 {
		t.Fatalf("expected 0 migrations on second run, got %d", appliedAgain)
	}
}

func TestMigrateRollsBackBrokenSQL(t *testing.T) {
	t.Parallel()

	dsn := "file:" + filepath.Join(t.TempDir(), "broken.db") + "?_pragma=foreign_keys(1)"
	db, err := database.Open(config.Config{DBDSN: dsn})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(db); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	fsys := fstest.MapFS{
		"0001_good.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE items (id TEXT PRIMARY KEY);`)},
		"0002_bad.sql":  &fstest.MapFile{Data: []byte(`CREATE TABLE broken (id TEXT); INVALID SQL;`)},
	}

	applied, err := database.Migrate(context.Background(), db, fsys)
	if err == nil {
		t.Fatal("expected error for broken SQL migration, got nil")
	}
	if applied != 1 {
		t.Fatalf("applied = %d, want 1", applied)
	}
}
