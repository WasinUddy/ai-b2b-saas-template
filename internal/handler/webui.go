package handler

import (
	"path"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
)

// registerWebUI serves the built frontend from dir. Files are served as they
// are. Any other GET or HEAD path without a file extension gets index.html so
// client-side routes survive a reload. Paths with an extension that match no
// file, and every non-GET method, fall through to the 404 handler.
func registerWebUI(app *fiber.App, dir string) {
	index := filepath.Join(dir, "index.html")
	app.Static("/", dir, fiber.Static{Index: "index.html"})
	app.Use(func(c *fiber.Ctx) error {
		if (c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead) || path.Ext(c.Path()) != "" {
			return c.Next()
		}
		return c.SendFile(index)
	})
}
