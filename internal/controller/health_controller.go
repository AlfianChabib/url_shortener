package controller

import (
	"net/http"
	"url_shortener/internal/helper/response"

	"github.com/gofiber/fiber/v2"
)

// HealthController provides health-check endpoint.
type HealthController interface {
	HealthCheck(c *fiber.Ctx) error
}

type healthControllerImpl struct{}

// NewHealthController creates a new HealthController instance.
func NewHealthController() HealthController {
	return &healthControllerImpl{}
}

// HealthCheck handles GET /health
func (ctrl *healthControllerImpl) HealthCheck(c *fiber.Ctx) error {
	return response.SuccessResponse(c, http.StatusOK, fiber.Map{
		"status":  "healthy",
		"version": "1.0.0",
	}, "Service is up and running")
}

