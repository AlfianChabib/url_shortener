package response

import (
	"github.com/gofiber/fiber/v2"
)

// ApiResponse represents the standard standardized JSON API response structure.
type ApiResponse[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    T      `json:"data,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}

// SuccessResponse sends a standard JSON success response.
func SuccessResponse(c *fiber.Ctx, statusCode int, data any, message ...string) error {
	msg := ""
	if len(message) > 0 {
		msg = message[0]
	}

	return c.Status(statusCode).JSON(ApiResponse[any]{
		Success: true,
		Message: msg,
		Data:    data,
	})
}

// ErrorResponse sends a standard JSON error response.
func ErrorResponse(c *fiber.Ctx, statusCode int, message string, errors ...any) error {
	var errDetails any
	if len(errors) > 0 {
		errDetails = errors[0]
	}

	return c.Status(statusCode).JSON(ApiResponse[any]{
		Success: false,
		Message: message,
		Errors:  errDetails,
	})
}

