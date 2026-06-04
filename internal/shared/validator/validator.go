package validator

import (
	"strings"

	"footwear-backend/internal/shared/response"
	govalidator "github.com/go-playground/validator/v10"
)

var Validate = govalidator.New()

func NormalizeErrors(err error) []response.ErrorItem {
	ve, ok := err.(govalidator.ValidationErrors)
	if !ok {
		return []response.ErrorItem{{Message: err.Error()}}
	}
	out := make([]response.ErrorItem, 0, len(ve))
	for _, item := range ve {
		out = append(out, response.ErrorItem{
			Field:   strings.ToLower(item.Field()),
			Message: messageFor(item.Tag(), item.Field()),
		})
	}
	return out
}

func messageFor(tag, field string) string {
	switch tag {
	case "required":
		return field + " is required"
	case "email":
		return "Email is invalid"
	case "min":
		return field + " is too short"
	case "gt":
		return field + " must be greater than zero"
	default:
		return field + " is invalid"
	}
}
