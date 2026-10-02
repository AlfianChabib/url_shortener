package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// NewRecoverMiddleware creates a recovery middleware that intercepts panics.
func NewRecoverMiddleware() fiber.Handler {
	return recover.New(recover.Config{
		EnableStackTrace: true,
	})
}

