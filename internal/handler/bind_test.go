package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

type sampleRequest struct {
	CheckInDate string `json:"checkInDate" validate:"required"`
	GuestCount  int    `json:"guestCount" validate:"omitempty,min=1"`
}

type sampleParams struct {
	PageSize int `query:"pageSize" validate:"required,min=1"`
}

func TestBindJSONFieldError(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{ErrorHandler: NewErrorHandler(discardLog())})
	app.Post("/bookings", func(c *fiber.Ctx) error {
		var req sampleRequest
		if err := bindJSON(c, &req); err != nil {
			return err
		}
		return respond(c, fiber.StatusCreated, req)
	})
	body := strings.NewReader(`{"guestCount":0}`)
	req := httptest.NewRequest(http.MethodPost, "/bookings", body)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	payload := decodeError(t, res)
	if payload.Error.Code != "INVALID_ARGUMENT" {
		t.Fatalf("code = %q", payload.Error.Code)
	}
	if len(payload.Error.Details) != 1 || payload.Error.Details[0].Field != "checkInDate" {
		t.Fatalf("details = %#v", payload.Error.Details)
	}
}

func TestBindQuery(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{ErrorHandler: NewErrorHandler(discardLog())})
	app.Get("/bookings", func(c *fiber.Ctx) error {
		var params sampleParams
		if err := bindQuery(c, &params); err != nil {
			return err
		}
		return noContent(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/bookings?pageSize=20", nil)
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", res.StatusCode)
	}
}

func decodeError(t *testing.T, res *http.Response) ErrorResponse {
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
	var payload ErrorResponse
	if decodeErr := unmarshalJSON(raw, &payload); decodeErr != nil {
		t.Fatalf("decode %s: %v", raw, decodeErr)
	}
	return payload
}
