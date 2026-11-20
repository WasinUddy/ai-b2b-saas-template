package model

import (
	"errors"
	"time"
)

// ErrInvalidToken indicates a missing, expired, or invalid auth credential.
var ErrInvalidToken = errors.New("invalid auth token")

// AuthClaims holds verified caller identity extracted by the auth middleware.
type AuthClaims struct {
	Subject   string
	Scopes    []string
	ExpiresAt time.Time
}
