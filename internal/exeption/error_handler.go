package exeption

import (
	"errors"
	"net/http"
	"url_shortener/internal/helper/response"
	"url_shortener/pkg/errs"
	"url_shortener/pkg/validator"

	val "github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// ErrorHandler provides a centralized error handling mechanism for Fiber.
func ErrorHandler(c *fiber.Ctx, err error) error {
	// Case 1: Custom AppError
	var appErr *errs.AppError
	if errors.As(err, &appErr) {
		return response.ErrorResponse(c, appErr.Code, appErr.Message)
	}

	// Case 2: Validation Errors from go-playground/validator
	var validationErrors val.ValidationErrors
	if errors.As(err, &validationErrors) {
		formatted := validator.FormatValidationErrors(validationErrors)
		return response.ErrorResponse(c, http.StatusBadRequest, "Validation error", formatted)
	}

	// Case 3: Fiber built-in errors (e.g. 404 Route Not Found, 405 Method Not Allowed)
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return response.ErrorResponse(c, fiberErr.Code, fiberErr.Message)
	}

	// Case 4: Default 500 Internal Server Error
	return response.ErrorResponse(c, http.StatusInternalServerError, "Internal Server Error", err.Error())
}

