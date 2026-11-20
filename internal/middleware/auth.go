package middleware

import (
	"context"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/wasinuddy/ai-b2b-saas-template/internal/model"
)

type authClaimsContextKey struct{}

// TokenVerifier validates a bearer token and returns caller claims.
// Implement this interface with your chosen auth strategy (JWT, session token,
// OAuth/OIDC provider, or API key) when enabling AUTH_ENABLED=true.
type TokenVerifier interface {
	VerifyToken(ctx context.Context, token string) (model.AuthClaims, error)
}

// RequireAuth protects routes with a Bearer token check when enabled is true.
// When enabled is false (the template default), requests pass through unchanged.
// When enabled is true and verifier is nil, every request is rejected with 401.
func RequireAuth(verifier TokenVerifier, enabled bool) fiber.Handler {
	if !enabled {
		return func(c *fiber.Ctx) error {
			return c.Next()
		}
	}
	return func(c *fiber.Ctx) error {
		if verifier == nil {
			return fmt.Errorf("auth verifier not configured: %w", model.ErrInvalidToken)
		}
		token, ok := bearerToken(c.Get(fiber.HeaderAuthorization))
		if !ok {
			return fmt.Errorf("authorization header: %w", model.ErrInvalidToken)
		}
		claims, err := verifier.VerifyToken(c.UserContext(), token)
		if err != nil {
			return fmt.Errorf("verify bearer token: %w", err)
		}
		c.Locals(authClaimsContextKey{}, claims)
		return c.Next()
	}
}

// AuthClaims extracts the verified caller claims stored by RequireAuth.
func AuthClaims(c *fiber.Ctx) (model.AuthClaims, bool) {
	claims, ok := c.Locals(authClaimsContextKey{}).(model.AuthClaims)
	return claims, ok
}

func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(strings.TrimSpace(header), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	return token, true
}
