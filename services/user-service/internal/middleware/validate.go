package middleware

import (
	"net/http"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

func Validate(w http.ResponseWriter, v any) bool {
	if err := validate.Struct(v); err != nil {
		WriteError(w, http.StatusUnprocessableEntity, err.Error())
		return false
	}
	return true
}
