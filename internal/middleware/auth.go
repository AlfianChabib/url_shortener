package middleware

import (
	"strings"
	"url_shortener/internal/repository"
	"url_shortener/pkg/errs"
	"url_shortener/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

// NewJWTMiddleware creates a middleware that enforces JWT authentication and verifies revocation.
func NewJWTMiddleware(jwtSecret string, blacklist ...repository.TokenBlacklistRepository) fiber.Handler {
	var bl repository.TokenBlacklistRepository
	if len(blacklist) > 0 {
		bl = blacklist[0]
	}

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

		// PRD Section 6.6: Check token blacklist
		if bl != nil {
			blacklisted, err := bl.IsTokenBlacklisted(c.Context(), tokenString)
			if err == nil && blacklisted {
				return errs.NewUnauthorizedError("token has been revoked / logged out")
			}
		}

		claims, err := utils.ValidateToken(tokenString, jwtSecret)
		if err != nil {
			return errs.NewUnauthorizedError("invalid or expired token", err)
		}

		// Store claims and token string in Fiber context locals
		c.Locals("user_id", claims.UserID)
		c.Locals("email", claims.Email)
		c.Locals("username", claims.Username)
		c.Locals("token_string", tokenString)

		return c.Next()
	}
}

// NewOptionalJWTMiddleware extracts user claims if JWT token is present, but permits unauthenticated access.
func NewOptionalJWTMiddleware(jwtSecret string, blacklist ...repository.TokenBlacklistRepository) fiber.Handler {
	var bl repository.TokenBlacklistRepository
	if len(blacklist) > 0 {
		bl = blacklist[0]
	}

	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				tokenString := strings.TrimSpace(parts[1])

				// If token is blacklisted, don't authenticate user
				isRevoked := false
				if bl != nil {
					blacklisted, err := bl.IsTokenBlacklisted(c.Context(), tokenString)
					if err == nil && blacklisted {
						isRevoked = true
					}
				}

				if !isRevoked {
					claims, err := utils.ValidateToken(tokenString, jwtSecret)
					if err == nil && claims != nil {
						c.Locals("user_id", claims.UserID)
						c.Locals("email", claims.Email)
						c.Locals("username", claims.Username)
						c.Locals("token_string", tokenString)
					}
				}
			}
		}

		return c.Next()
	}
}
