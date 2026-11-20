// Package handler adapts HTTP requests and responses to service calls.
//
// handler may import service, model, and middleware, and it may use Fiber.
// It must not import repository or platform, and it must not contain business
// rules or database access.
package handler
