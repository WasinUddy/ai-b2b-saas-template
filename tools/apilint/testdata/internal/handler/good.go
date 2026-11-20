package handler

import (
	"example.com/fiber"
	"example.com/internal/model"
)

type HealthResponse struct {
	Status string `json:"status"`
}

type CreateBookingRequest struct {
	CheckInDate string `json:"checkInDate" validate:"required"`
	Note        string `json:"note,omitempty" validate:"omitempty,max=10"`
}

type ListBookingParams struct {
	PageSize int `query:"pageSize" validate:"omitempty,min=1"`
}

type HealthHandler struct{}

func respond(_ *fiber.Ctx, _ int, _ any) error { return nil }
func bindJSON(_ *fiber.Ctx, _ any) error       { return nil }
func bindQuery(_ *fiber.Ctx, _ any) error      { return nil }

func ok(status model.HealthStatus) {
	respond(nil, 200, HealthResponse{Status: status.Status})
	req := CreateBookingRequest{}
	bindJSON(nil, &req)
	params := ListBookingParams{}
	bindQuery(nil, &params)
}

// Ready is annotated, so apilint leaves it alone.
//
// @Router /readyz [get]
// @Success 200 {object} HealthResponse
// @Failure 503 {object} ErrorResponse
func (h *HealthHandler) Ready(c *fiber.Ctx) error {
	return respond(c, 200, HealthResponse{})
}

type wrapped struct{ cause error }

func (w wrapped) Error() string { return w.cause.Error() }

func keepsCause(err error) error {
	if err != nil {
		return wrapped{cause: err}
	}
	if count := 1; count != 0 {
		return nil
	}
	return nil
}

func (h *HealthHandler) hidden(_ *fiber.Ctx) error { return nil }

func Live(_ *fiber.Ctx) error { return nil }

func (h *HealthHandler) Ping() error { return nil }
