package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/wasinuddy/ai-b2b-saas-template/internal/middleware"
	"github.com/wasinuddy/ai-b2b-saas-template/internal/model"
)

type fakeVerifier struct {
	claims model.AuthClaims
	err    error
}

func (f fakeVerifier) VerifyToken(_ context.Context, _ string) (model.AuthClaims, error) {
	if f.err != nil {
		return model.AuthClaims{}, f.err
	}
	return f.claims, nil
}

func TestRequireAuthDisabledPassesThrough(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/protected", middleware.RequireAuth(nil, false), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/protected", nil))
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
}

func TestRequireAuthEnabledFailsClosedWithoutVerifier(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			if errors.Is(err, model.ErrInvalidToken) {
				return c.SendStatus(fiber.StatusUnauthorized)
			}
			return c.SendStatus(fiber.StatusInternalServerError)
		},
	})
	app.Get("/protected", middleware.RequireAuth(nil, true), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer any-token")
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", res.StatusCode)
	}
}

func TestRequireAuthEnabledChecksBearerToken(t *testing.T) {
	t.Parallel()

	wantClaims := model.AuthClaims{Subject: "user-1", Scopes: []string{"read"}}
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			if errors.Is(err, model.ErrInvalidToken) {
				return c.SendStatus(fiber.StatusUnauthorized)
			}
			return c.SendStatus(fiber.StatusInternalServerError)
		},
	})
	app.Get("/protected", middleware.RequireAuth(fakeVerifier{claims: wantClaims}, true), func(c *fiber.Ctx) error {
		got, ok := middleware.AuthClaims(c)
		if !ok || got.Subject != wantClaims.Subject {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	unauthRes, err := app.Test(httptest.NewRequest(http.MethodGet, "/protected", nil))
	if err != nil {
		t.Fatalf("unauth Test() error = %v", err)
	}
	if unauthRes.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing header status = %d, want 401", unauthRes.StatusCode)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	okRes, err := app.Test(req)
	if err != nil {
		t.Fatalf("auth Test() error = %v", err)
	}
	if okRes.StatusCode != http.StatusOK {
		t.Fatalf("valid token status = %d, want 200", okRes.StatusCode)
	}
}
