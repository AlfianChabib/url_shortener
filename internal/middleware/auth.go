package middleware

import (
	"strings"
	"url_shortener/pkg/errs"
	"url_shortener/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

// NewJWTMiddleware creates a middleware that enforces JWT authentication.
func NewJWTMiddleware(jwtSecret string) fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return errs.NewUnauthorizedError("missing authorization header")
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return errs.NewUnauthorizedError("invalid authorization header format, expected 'Bearer <token>'")
		}

		tokenString := strings.TrimSpace(parts[1])
		claims, err := utils.ValidateToken(tokenString, jwtSecret)
		if err != nil {
			return errs.NewUnauthorizedError("invalid or expired token", err)
		}

		// Store claims in Fiber context locals
		c.Locals("user_id", claims.UserID)
		c.Locals("email", claims.Email)
		c.Locals("username", claims.Username)

		return c.Next()
	}
}

// NewOptionalJWTMiddleware extracts user claims if JWT token is present, but permits unauthenticated access.
func NewOptionalJWTMiddleware(jwtSecret string) fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				tokenString := strings.TrimSpace(parts[1])
				claims, err := utils.ValidateToken(tokenString, jwtSecret)
				if err == nil && claims != nil {
					c.Locals("user_id", claims.UserID)
					c.Locals("email", claims.Email)
					c.Locals("username", claims.Username)
				}
			}
		}

		return c.Next()
	}
}
