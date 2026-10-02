package router

import (
	"url_shortener/internal/controller"
	"url_shortener/internal/middleware"

	"github.com/gofiber/fiber/v3"
)

// SetupRouter registers all application routes and middlewares to the Fiber app.
func SetupRouter(
	app *fiber.App,
	linkController controller.LinkController,
	healthController controller.HealthController,
) {
	// Global Middlewares
	app.Use(middleware.NewRecoverMiddleware())
	app.Use(middleware.NewLoggerMiddleware())
	app.Use(middleware.NewCORSMiddleware())
	app.Use(middleware.NewSecurityHeadersMiddleware())

	// Health Check
	app.Get("/health", healthController.HealthCheck)

	// API v1 routes
	api := app.Group("/api/v1")
	{
		links := api.Group("/links")
		links.Post("/", linkController.Create)
		links.Get("/:short_code/analytics", linkController.GetAnalytics)
	}

	// High-Throughput Redirection Route
	app.Get("/:short_code", linkController.Redirect)
}
