package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

type sseTestEvent struct {
	Token string `json:"token"`
}

func TestStreamSSEWritesFramesAndHeaders(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/api/v1/test-stream", func(c *fiber.Ctx) error {
		return streamSSE(c, discardLog(), func(w *SSEWriter) error {
			if err := w.Comment("connected"); err != nil {
				return err
			}
			if err := w.Send("delta", sseTestEvent{Token: "hello"}); err != nil {
				return err
			}
			return w.SendText("done", "ok")
		})
	})

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/test-stream", nil))
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if got := res.Header.Get(fiber.HeaderContentType); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
	body := string(readAll(t, res))
	want := ": connected\n\nevent: delta\ndata: {\"token\":\"hello\"}\n\nevent: done\ndata: ok\n\n"
	if body != want {
		t.Fatalf("body = %q, want %q", body, want)
	}
}
