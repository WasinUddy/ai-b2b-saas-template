package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/wasinuddy/ai-b2b-saas-template/internal/model"
)

func TestErrorHandlerHidesInternalError(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{ErrorHandler: NewErrorHandler(discardLog())})
	app.Get("/", func(_ *fiber.Ctx) error {
		return errors.New("sql: password=secret")
	})

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", res.StatusCode)
	}
	raw := readAll(t, res)
	if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "sql:") {
		t.Fatalf("body leaked internal error: %s", raw)
	}
	var payload ErrorResponse
	if decodeErr := json.Unmarshal(raw, &payload); decodeErr != nil {
		t.Fatalf("decode: %v", decodeErr)
	}
	if payload.Error.Code != "INTERNAL" || payload.Error.Message != internalErrorMessage {
		t.Fatalf("body = %#v", payload.Error)
	}
}

func TestErrorHandlerLogsEveryError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		err    error
		status int
		level  string
		cause  string
	}{
		{
			name:   "client error keeps cause",
			err:    newAPIError(http.StatusBadRequest, "The request is invalid.", errors.New("unexpected EOF")),
			status: http.StatusBadRequest,
			level:  "level=WARN",
			cause:  "unexpected EOF",
		},
		{
			name:   "unavailable keeps cause",
			err:    newAPIError(http.StatusServiceUnavailable, "not ready", errors.New("ping sqlite: closed")),
			status: http.StatusServiceUnavailable,
			level:  "level=ERROR",
			cause:  "ping sqlite: closed",
		},
		{
			name:   "unknown error",
			err:    errors.New("sql: password=secret"),
			status: http.StatusInternalServerError,
			level:  "level=ERROR",
			cause:  "sql: password=secret",
		},
		{
			name:   "fiber error",
			err:    fiber.NewError(fiber.StatusNotFound, "not found"),
			status: http.StatusNotFound,
			level:  "level=WARN",
			cause:  "not found",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var logs strings.Builder
			app := fiber.New(fiber.Config{ErrorHandler: NewErrorHandler(textLog(&logs))})
			app.Get("/", func(_ *fiber.Ctx) error { return tc.err })

			res, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
			if err != nil {
				t.Fatalf("Test() error = %v", err)
			}
			if res.StatusCode != tc.status {
				t.Fatalf("status = %d, want %d", res.StatusCode, tc.status)
			}
			if body := readAll(t, res); strings.Contains(string(body), "secret") {
				t.Fatalf("body leaked cause: %s", body)
			}
			got := logs.String()
			if !strings.Contains(got, tc.level) || !strings.Contains(got, tc.cause) {
				t.Fatalf("log = %q, want %s with %q", got, tc.level, tc.cause)
			}
		})
	}
}

func TestPanicIsLoggedAsInternalError(t *testing.T) {
	t.Parallel()

	var logs strings.Builder
	log := textLog(&logs)
	app := fiber.New(fiber.Config{ErrorHandler: NewErrorHandler(log)})
	Register(app, Dependencies{
		Health: &HealthHandler{health: panicHealth{}},
		Log:    log,
	})

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", res.StatusCode)
	}
	got := logs.String()
	if !strings.Contains(got, "level=ERROR") || !strings.Contains(got, "boom") {
		t.Fatalf("log = %q, want the panic logged at error level", got)
	}
	if !strings.Contains(got, "status=500") {
		t.Fatalf("log = %q, want the request line to report status 500", got)
	}
}

type panicHealth struct{}

func (panicHealth) Ready(context.Context) (model.HealthStatus, error) {
	panic("boom")
}

func textLog(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, nil))
}

func TestCodeForStatus(t *testing.T) {
	t.Parallel()

	cases := map[int]string{
		http.StatusBadRequest:            "INVALID_ARGUMENT",
		http.StatusNotFound:              "NOT_FOUND",
		http.StatusPreconditionFailed:    "FAILED_PRECONDITION",
		http.StatusRequestEntityTooLarge: "PAYLOAD_TOO_LARGE",
		http.StatusTooManyRequests:       "RESOURCE_EXHAUSTED",
		http.StatusTeapot:                "INTERNAL",
	}
	for status, want := range cases {
		if got := codeForStatus(status); got != want {
			t.Errorf("codeForStatus(%d) = %s, want %s", status, got, want)
		}
	}
}

func discardLog() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func unmarshalJSON(raw []byte, dst any) error {
	return json.Unmarshal(raw, dst)
}

func readAll(t *testing.T, res *http.Response) []byte {
	t.Helper()
	defer func() {
		if err := res.Body.Close(); err != nil {
			t.Errorf("close body: %v", err)
		}
	}()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return raw
}
