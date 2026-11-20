package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func newWebUIApp(t *testing.T) *fiber.App {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	files := map[string]string{
		"index.html":    "<html>spa</html>",
		"assets/app.js": "console.log(1)",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          NewErrorHandler(discardLog()),
	})
	Register(app, Dependencies{
		Health:   &HealthHandler{health: fakeHealth{}},
		Log:      discardLog(),
		WebUIDir: dir,
	})
	return app
}

func TestWebUI(t *testing.T) {
	t.Parallel()

	app := newWebUIApp(t)
	cases := []struct {
		name   string
		method string
		target string
		status int
		body   string
	}{
		{name: "root", method: http.MethodGet, target: "/", status: http.StatusOK, body: "spa"},
		{name: "asset", method: http.MethodGet, target: "/assets/app.js", status: http.StatusOK, body: "console.log"},
		{name: "client route", method: http.MethodGet, target: "/some/page", status: http.StatusOK, body: "spa"},
		{name: "missing asset", method: http.MethodGet, target: "/assets/missing.js", status: http.StatusNotFound, body: "NOT_FOUND"},
		{name: "api typo", method: http.MethodGet, target: "/api/v1/nope", status: http.StatusNotFound, body: "NOT_FOUND"},
		{name: "api root", method: http.MethodGet, target: "/api", status: http.StatusNotFound, body: "NOT_FOUND"},
		{name: "health wins", method: http.MethodGet, target: "/healthz", status: http.StatusOK, body: `"status":"ok"`},
		{name: "post unknown", method: http.MethodPost, target: "/some/page", status: http.StatusNotFound, body: "NOT_FOUND"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res, err := app.Test(httptest.NewRequest(tc.method, tc.target, nil))
			if err != nil {
				t.Fatalf("Test() error = %v", err)
			}
			body := string(readAll(t, res))
			if res.StatusCode != tc.status || !strings.Contains(body, tc.body) {
				t.Fatalf("got %d %q, want %d containing %q", res.StatusCode, body, tc.status, tc.body)
			}
		})
	}
}
