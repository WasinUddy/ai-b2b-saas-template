package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gofiber/fiber/v2"

	"github.com/wasinuddy/ai-b2b-saas-template/internal/model"
)

const internalErrorMessage = "The server could not complete the request."

// ErrorDetail is one field-level problem inside an error response.
type ErrorDetail struct {
	Field  string `json:"field,omitempty"`
	Reason string `json:"reason"`
}

// ErrorBody is the stable error object returned to clients.
type ErrorBody struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`
}

// ErrorResponse is the envelope for every unsuccessful response.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// APIError is a client-safe failure with an HTTP status and stable code.
// Cause is logged but never sent to the client.
type APIError struct {
	Status  int
	Code    string
	Message string
	Details []ErrorDetail
	Cause   error
}

func newAPIError(status int, message string, cause error) *APIError {
	return &APIError{Status: status, Code: codeForStatus(status), Message: message, Cause: cause}
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = e.Code
	}
	if e.Cause != nil {
		return msg + ": " + e.Cause.Error()
	}
	return msg
}

func (e *APIError) Unwrap() error {
	return e.Cause
}

// NewErrorHandler logs every failure once and renders it as ErrorResponse JSON.
// 5xx failures log at error level, everything else at warn. Unknown errors are
// replaced with a generic 500 message.
func NewErrorHandler(log *slog.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		status, body := errorResponse(err)
		level := slog.LevelWarn
		if status >= http.StatusInternalServerError {
			level = slog.LevelError
		}
		log.LogAttrs(c.UserContext(), level, "request failed",
			slog.String("err", err.Error()),
			slog.String("code", body.Error.Code),
			slog.Int("status", status),
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
		)
		return respond(c, status, body)
	}
}

func errorResponse(err error) (int, ErrorResponse) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status, ErrorResponse{Error: ErrorBody{
			Code:    apiErr.Code,
			Message: apiErr.Message,
			Details: apiErr.Details,
		}}
	}

	if errors.Is(err, model.ErrInvalidToken) {
		return http.StatusUnauthorized, ErrorResponse{Error: ErrorBody{
			Code:    codeForStatus(http.StatusUnauthorized),
			Message: "authentication required",
		}}
	}

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return fiberErr.Code, ErrorResponse{Error: ErrorBody{
			Code:    codeForStatus(fiberErr.Code),
			Message: fiberErr.Message,
		}}
	}

	return http.StatusInternalServerError, ErrorResponse{Error: ErrorBody{
		Code:    "INTERNAL",
		Message: internalErrorMessage,
	}}
}

func codeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "INVALID_ARGUMENT"
	case http.StatusUnauthorized:
		return "UNAUTHENTICATED"
	case http.StatusForbidden:
		return "PERMISSION_DENIED"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusUnprocessableEntity:
		return "UNPROCESSABLE"
	case http.StatusPreconditionFailed:
		return "FAILED_PRECONDITION"
	case http.StatusRequestEntityTooLarge:
		return "PAYLOAD_TOO_LARGE"
	case http.StatusTooManyRequests:
		return "RESOURCE_EXHAUSTED"
	case http.StatusServiceUnavailable:
		return "UNAVAILABLE"
	default:
		return "INTERNAL"
	}
}
