package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"url_shortener/internal/exeption"
	"url_shortener/internal/middleware"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestSlidingWindowLimiter_Allow(t *testing.T) {
	limiter := middleware.NewRateLimiter(nil)
	ctx := context.Background()

	key := "test:user:123"
	limit := int64(3)
	window := 200 * time.Millisecond

	// 1st request -> Allowed
	res, err := limiter.Allow(ctx, key, limit, window)
	assert.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, int64(2), res.Remaining)

	// 2nd request -> Allowed
	res, err = limiter.Allow(ctx, key, limit, window)
	assert.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, int64(1), res.Remaining)

	// 3rd request -> Allowed
	res, err = limiter.Allow(ctx, key, limit, window)
	assert.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, int64(0), res.Remaining)

	// 4th request -> Denied (429)
	res, err = limiter.Allow(ctx, key, limit, window)
	assert.NoError(t, err)
	assert.False(t, res.Allowed)
	assert.Equal(t, int64(0), res.Remaining)

	// Wait for window to expire
	time.Sleep(250 * time.Millisecond)

	// 5th request -> Allowed again
	res, err = limiter.Allow(ctx, key, limit, window)
	assert.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, int64(2), res.Remaining)
}

func TestSubnet24Extraction(t *testing.T) {
	tests := []struct {
		ip       string
		expected string
	}{
		{"192.168.1.150", "192.168.1.0/24"},
		{"10.20.30.40", "10.20.30.0/24"},
		{"::1", "::1"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, middleware.GetIPv4Subnet24(tt.ip))
	}
}

func TestLinkCreationRateLimiter_Anonymous(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: exeption.ErrorHandler})
	limiter := middleware.NewRateLimiter(nil)

	app.Post("/test-links", middleware.NewLinkCreationRateLimiter(limiter), func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusCreated)
	})

	// Make 10 requests: all 10 should succeed (limit 10/min)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test-links", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, "10", resp.Header.Get("X-RateLimit-Limit"))
	}

	// 11th request: must be rejected with 429 Too Many Requests
	req := httptest.NewRequest(http.MethodPost, "/test-links", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("Retry-After"))
}

func TestLinkCreationRateLimiter_Authenticated(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: exeption.ErrorHandler})
	limiter := middleware.NewRateLimiter(nil)

	testUserID := uuid.New()

	// Inject authenticated user into locals
	app.Use(func(c fiber.Ctx) error {
		c.Locals("user_id", testUserID)
		return c.Next()
	})
	app.Post("/test-links", middleware.NewLinkCreationRateLimiter(limiter), func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusCreated)
	})

	// 1st request has limit 1000
	req := httptest.NewRequest(http.MethodPost, "/test-links", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "1000", resp.Header.Get("X-RateLimit-Limit"))
}

func TestAuthLoginRateLimiter(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: exeption.ErrorHandler})
	limiter := middleware.NewRateLimiter(nil)

	app.Post("/login", middleware.NewAuthLoginRateLimiter(limiter), func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})

	// 5 attempts allowed
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}

	// 6th attempt rejected
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
}

func TestAuthRegisterRateLimiter(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: exeption.ErrorHandler})
	limiter := middleware.NewRateLimiter(nil)

	app.Post("/register", middleware.NewAuthRegisterRateLimiter(limiter), func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusCreated)
	})

	// 3 registrations allowed
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/register", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	}

	// 4th registration rejected
	req := httptest.NewRequest(http.MethodPost, "/register", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
}
