package service

import (
	"context"
	"errors"
	"testing"
)

type fakeHealthRepository struct {
	err error
}

func (f fakeHealthRepository) Ping(_ context.Context) error {
	return f.err
}

func TestReady(t *testing.T) {
	t.Parallel()

	svc := NewHealthService(fakeHealthRepository{})
	got, err := svc.Ready(context.Background())
	if err != nil {
		t.Fatalf("Ready() error = %v", err)
	}
	if got.Status != "ok" {
		t.Fatalf("Ready() status = %q, want ok", got.Status)
	}
}

func TestReadyDatabaseDown(t *testing.T) {
	t.Parallel()

	svc := NewHealthService(fakeHealthRepository{err: errors.New("database down")})
	if _, err := svc.Ready(context.Background()); err == nil {
		t.Fatal("Ready() error = nil, want failure")
	}
}
