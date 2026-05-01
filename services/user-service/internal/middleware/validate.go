package middleware

import (
	"net/http"
	"regexp"

	"github.com/go-playground/validator/v10"
)

var phoneRx = regexp.MustCompile(`^\+?[0-9][\d\s\-\(\)]{5,18}$`)
var strongPasswordRx = regexp.MustCompile(`^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).+$`)

var validate = func() *validator.Validate {
	v := validator.New()
	_ = v.RegisterValidation("phone", func(fl validator.FieldLevel) bool {
		return phoneRx.MatchString(fl.Field().String())
	})
	_ = v.RegisterValidation("strongpassword", func(fl validator.FieldLevel) bool {
		return strongPasswordRx.MatchString(fl.Field().String())
	})
	return v
}()

func Validate(w http.ResponseWriter, v any) bool {
	if err := validate.Struct(v); err != nil {
		WriteError(w, http.StatusUnprocessableEntity, "validation failed: check required fields and format")
		return false
	}
	return true
}
