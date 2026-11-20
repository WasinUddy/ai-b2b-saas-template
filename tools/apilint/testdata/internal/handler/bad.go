package handler

import (
	"example.com/fiber"
	"example.com/internal/model"
)

type BadRequest struct {
	CheckInDate string // want `missing json or query tag` `must include required or omitempty`
}

type MismatchRequest struct {
	Note  string `json:"note" validate:"omitempty,max=10"`    // want `json omitempty and validate omitempty must agree`
	Other string `json:"other,omitempty" validate:"required"` // want `json omitempty and validate omitempty must agree`
}

type BadResponse struct {
	GuestCount int // want `response field needs a json tag`
}

type Booking struct {
	ID string `json:"id" validate:"required"`
}

func badCalls() {
	respond(nil, 200, map[string]string{})   // want `dto-only-responses`
	respond(nil, 200, model.HealthStatus{})  // want `dto-only-responses`
	respond(nil, 200, struct{ N int }{N: 1}) // want `dto-only-responses`
	bindJSON(nil, &model.HealthStatus{})     // want `dto-only-requests`
	bindJSON(nil, &Booking{})                // want `dto-only-requests`
}

func dropsCause(err error) error {
	if err != nil { // want `error-cause: err is dropped`
		return &Booking{}
	}
	return nil
}

func (b *Booking) Error() string { return b.ID }

func (h *HealthHandler) Live(_ *fiber.Ctx) error { // want `handler-annotated`
	return nil
}
