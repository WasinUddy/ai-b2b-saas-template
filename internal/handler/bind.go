package handler

import (
	"errors"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New()
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		for _, key := range []string{"json", "query"} {
			name := strings.Split(field.Tag.Get(key), ",")[0]
			if name != "" && name != "-" {
				return name
			}
		}
		return field.Name
	})
	return v
}

func bindJSON(c *fiber.Ctx, dst any) error {
	if err := c.BodyParser(dst); err != nil {
		return invalidArgument(err)
	}
	return validateInput(dst)
}

func bindQuery(c *fiber.Ctx, dst any) error {
	if err := c.QueryParser(dst); err != nil {
		return invalidArgument(err)
	}
	return validateInput(dst)
}

func validateInput(dst any) error {
	if err := validate.Struct(dst); err != nil {
		return invalidArgument(err)
	}
	return nil
}

func invalidArgument(err error) error {
	apiErr := newAPIError(http.StatusBadRequest, "The request is invalid.", err)
	var fieldErrs validator.ValidationErrors
	if !errors.As(err, &fieldErrs) {
		return apiErr
	}
	apiErr.Details = make([]ErrorDetail, 0, len(fieldErrs))
	for _, fieldErr := range fieldErrs {
		apiErr.Details = append(apiErr.Details, ErrorDetail{
			Field:  fieldErr.Field(),
			Reason: validationReason(fieldErr),
		})
	}
	return apiErr
}

func validationReason(fieldErr validator.FieldError) string {
	switch fieldErr.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email"
	default:
		return "failed on " + fieldErr.Tag()
	}
}
