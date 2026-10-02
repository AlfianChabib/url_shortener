package errs

import (
	"fmt"
	"net/http"
)

// AppError represents a custom application error with HTTP status code
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// NewNotFoundError creates a 404 Not Found error
func NewNotFoundError(message string, err ...error) *AppError {
	var cause error
	if len(err) > 0 {
		cause = err[0]
	}
	return &AppError{
		Code:    http.StatusNotFound,
		Message: message,
		Err:     cause,
	}
}

// NewBadRequestError creates a 400 Bad Request error
func NewBadRequestError(message string, err ...error) *AppError {
	var cause error
	if len(err) > 0 {
		cause = err[0]
	}
	return &AppError{
		Code:    http.StatusBadRequest,
		Message: message,
		Err:     cause,
	}
}

// NewConflictError creates a 409 Conflict error
func NewConflictError(message string, err ...error) *AppError {
	var cause error
	if len(err) > 0 {
		cause = err[0]
	}
	return &AppError{
		Code:    http.StatusConflict,
		Message: message,
		Err:     cause,
	}
}

// NewGoneError creates a 410 Gone error (e.g. expired link)
func NewGoneError(message string, err ...error) *AppError {
	var cause error
	if len(err) > 0 {
		cause = err[0]
	}
	return &AppError{
		Code:    http.StatusGone,
		Message: message,
		Err:     cause,
	}
}

// NewInternalError creates a 500 Internal Server Error
func NewInternalError(message string, err ...error) *AppError {
	var cause error
	if len(err) > 0 {
		cause = err[0]
	}
	return &AppError{
		Code:    http.StatusInternalServerError,
		Message: message,
		Err:     cause,
	}
}

