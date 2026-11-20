// Package gormrepo implements repository interfaces with GORM on SQLite.
//
// gormrepo may import repository and model, and it may import GORM. It must
// not import handler, middleware, or service. Schema changes belong in
// migrations as SQL, not in GORM AutoMigrate.
package gormrepo
