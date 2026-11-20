package repository

import "context"

// HealthRepository checks that the database is reachable.
type HealthRepository interface {
	Ping(ctx context.Context) error
}
