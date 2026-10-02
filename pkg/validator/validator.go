package validator

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// CustomValidator wraps go-playground validator
type CustomValidator struct {
	validator *validator.Validate
}

// NewValidator initializes a new CustomValidator instance
func NewValidator() *CustomValidator {
	v := validator.New()
	return &CustomValidator{validator: v}
}

// GetValidator returns the underlying validator instance
func (cv *CustomValidator) GetValidator() *validator.Validate {
	return cv.validator
}

// ValidateStruct validates a struct and returns formatted error messages if invalid
func (cv *CustomValidator) ValidateStruct(s interface{}) error {
	return cv.validator.Struct(s)
}

// FormatValidationErrors formats validator.ValidationErrors into a human-readable map
func FormatValidationErrors(err error) map[string]string {
	res := make(map[string]string)
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, fe := range validationErrors {
			field := strings.ToLower(fe.Field())
			switch fe.Tag() {
			case "required":
				res[field] = fmt.Sprintf("%s is required", fe.Field())
			case "url":
				res[field] = fmt.Sprintf("%s must be a valid URL", fe.Field())
			case "min":
				res[field] = fmt.Sprintf("%s must be at least %s characters", fe.Field(), fe.Param())
			case "max":
				res[field] = fmt.Sprintf("%s must not exceed %s characters", fe.Field(), fe.Param())
			default:
				res[field] = fmt.Sprintf("%s failed validation on '%s'", fe.Field(), fe.Tag())
			}
		}
	} else if err != nil {
		res["error"] = err.Error()
	}
	return res
}

