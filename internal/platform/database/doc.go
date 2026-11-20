// Package database opens and closes the SQLite connection.
//
// database is part of platform. It is one of the two packages that may import
// GORM or the SQLite driver. It must not import handler, middleware, or service.
package database
