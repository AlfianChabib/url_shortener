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
	authController controller.AuthController,
	healthController controller.HealthController,
	jwtSecret string,
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
		// Auth routes
		auth := api.Group("/auth")
		auth.Post("/register", authController.Register)
		auth.Post("/login", authController.Login)
		auth.Get("/me", middleware.NewJWTMiddleware(jwtSecret), authController.GetProfile)

		// Links routes (with optional JWT auth to identify user)
		links := api.Group("/links")
		links.Post("/", middleware.NewOptionalJWTMiddleware(jwtSecret), linkController.Create)
		links.Get("/:short_code/analytics", linkController.GetAnalytics)
	}

	// High-Throughput Redirection Route
	app.Get("/:short_code", linkController.Redirect)
}
