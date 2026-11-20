// Package service implements use cases and business rules.
//
// service may import repository and model. It must not import handler,
// middleware, or platform. Depend on repository interfaces, never on a
// database driver or a GORM implementation.
//
// Unit tests for this package must keep statement coverage at or above 85%.
package service
