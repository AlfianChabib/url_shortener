package middleware

import (
	"github.com/gofiber/fiber/v3"
)

// NewSecurityHeadersMiddleware injects hardened HTTP security headers according to PRD section 6.5.
func NewSecurityHeadersMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("Content-Security-Policy", "default-src 'none'")
		c.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		return c.Next()
	}
}
