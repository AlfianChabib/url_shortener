package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"url_shortener/internal/middleware"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.NewSecurityHeadersMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "default-src 'none'", resp.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "max-age=63072000; includeSubDomains; preload", resp.Header.Get("Strict-Transport-Security"))
	assert.Equal(t, "1; mode=block", resp.Header.Get("X-XSS-Protection"))
}
