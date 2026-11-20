package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// SSEWriter writes Server-Sent Events frames onto a streaming HTTP response.
type SSEWriter struct {
	bw *bufio.Writer
}

// Send encodes payload as JSON and writes a single SSE event frame.
func (w *SSEWriter) Send(event string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sse payload: %w", err)
	}
	return w.SendText(event, string(raw))
}

// SendText writes a named SSE event frame with a text payload and flushes it immediately.
func (w *SSEWriter) SendText(event, data string) error {
	if event != "" {
		if _, err := fmt.Fprintf(w.bw, "event: %s\n", event); err != nil {
			return fmt.Errorf("write sse event: %w", err)
		}
	}
	for _, line := range strings.Split(data, "\n") {
		if _, err := fmt.Fprintf(w.bw, "data: %s\n", line); err != nil {
			return fmt.Errorf("write sse data: %w", err)
		}
	}
	if _, err := w.bw.WriteString("\n"); err != nil {
		return fmt.Errorf("terminate sse frame: %w", err)
	}
	if err := w.bw.Flush(); err != nil {
		return fmt.Errorf("flush sse frame: %w", err)
	}
	return nil
}

// Comment writes an SSE keep-alive comment frame and flushes it immediately.
func (w *SSEWriter) Comment(text string) error {
	if _, err := fmt.Fprintf(w.bw, ": %s\n\n", text); err != nil {
		return fmt.Errorf("write sse comment: %w", err)
	}
	if err := w.bw.Flush(); err != nil {
		return fmt.Errorf("flush sse comment: %w", err)
	}
	return nil
}

// streamSSE configures Server-Sent Events headers and runs write inside Fiber's
// streaming body writer. Validate inputs and auth before calling streamSSE so
// errors still return the standard JSON ErrorResponse.
func streamSSE(c *fiber.Ctx, log *slog.Logger, write func(w *SSEWriter) error) error {
	c.Set(fiber.HeaderContentType, "text/event-stream")
	c.Set(fiber.HeaderCacheControl, "no-cache")
	c.Set(fiber.HeaderConnection, "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	c.Context().SetBodyStreamWriter(func(bw *bufio.Writer) {
		if err := write(&SSEWriter{bw: bw}); err != nil && log != nil {
			log.Warn("sse stream ended with error", slog.String("err", err.Error()))
		}
	})
	return nil
}
