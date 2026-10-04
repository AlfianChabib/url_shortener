package validator

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
)

var (
	usernameRegex    = regexp.MustCompile(`^[a-zA-Z0-9_.]{3,50}$`)
	customAliasRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{4,32}$`)
)

// CustomValidator wraps go-playground validator
type CustomValidator struct {
	validator *validator.Validate
}

// NewValidator initializes a new CustomValidator instance with enterprise security rules
func NewValidator() *CustomValidator {
	v := validator.New()

	// Register custom username policy: ^[a-zA-Z0-9_.]{3,50}$
	_ = v.RegisterValidation("username_policy", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		return usernameRegex.MatchString(val)
	})

	// Register enterprise password policy: 8-72 chars, min 1 uppercase, min 1 lowercase, min 1 number
	_ = v.RegisterValidation("password_policy", func(fl validator.FieldLevel) bool {
		pass := fl.Field().String()
		if len(pass) < 8 || len(pass) > 72 {
			return false
		}
		var hasUpper, hasLower, hasDigit bool
		for _, r := range pass {
			switch {
			case unicode.IsUpper(r):
				hasUpper = true
			case unicode.IsLower(r):
				hasLower = true
			case unicode.IsDigit(r):
				hasDigit = true
			}
		}
		return hasUpper && hasLower && hasDigit
	})

	// Register custom alias policy: ^[a-zA-Z0-9_-]{4,32}$
	_ = v.RegisterValidation("custom_alias_policy", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		return customAliasRegex.MatchString(val)
	})

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
			case "email":
				res[field] = fmt.Sprintf("%s must be a valid email address", fe.Field())
			case "min":
				res[field] = fmt.Sprintf("%s must be at least %s characters", fe.Field(), fe.Param())
			case "max":
				res[field] = fmt.Sprintf("%s must not exceed %s characters", fe.Field(), fe.Param())
			case "username_policy":
				res[field] = fmt.Sprintf("%s must be 3-50 alphanumeric characters (dots and underscores permitted)", fe.Field())
			case "password_policy":
				res[field] = fmt.Sprintf("%s must be 8-72 characters long and contain at least one uppercase letter, one lowercase letter, and one number", fe.Field())
			case "custom_alias_policy":
				res[field] = fmt.Sprintf("%s must be 4-32 alphanumeric characters, hyphens, or underscores", fe.Field())
			default:
				res[field] = fmt.Sprintf("%s failed validation on '%s'", fe.Field(), fe.Tag())
			}
		}
	} else if err != nil {
		res["error"] = err.Error()
	}
	return res
}

