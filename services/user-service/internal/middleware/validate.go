package middleware

import (
	"net/http"
	"regexp"
	"unicode"

	"github.com/go-playground/validator/v10"
)

var phoneRx = regexp.MustCompile(`^\+?[0-9][\d\s\-\(\)]{5,18}$`)

func isStrongPassword(s string) bool {
	var hasLower, hasUpper, hasDigit bool
	for _, r := range s {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	return hasLower && hasUpper && hasDigit && len(s) >= 1
}

var validate = func() *validator.Validate {
	v := validator.New()
	_ = v.RegisterValidation("phone", func(fl validator.FieldLevel) bool {
		return phoneRx.MatchString(fl.Field().String())
	})
	_ = v.RegisterValidation("strongpassword", func(fl validator.FieldLevel) bool {
		return isStrongPassword(fl.Field().String())
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
