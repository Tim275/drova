package middleware

import (
	"net/http"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

func Validate(w http.ResponseWriter, v any) bool {
	if err := validate.Struct(v); err != nil {
		WriteError(w, http.StatusUnprocessableEntity, "validation failed: check required fields and format")
		return false
	}
	return true
}
