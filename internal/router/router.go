package router

import (
	"url_shortener/internal/controller"
	"url_shortener/internal/middleware"
	"url_shortener/internal/repository"

	"github.com/gofiber/fiber/v3"
)

// SetupRouter registers all application routes and middlewares to the Fiber app.
func SetupRouter(
	app *fiber.App,
	linkController controller.LinkController,
	authController controller.AuthController,
	healthController controller.HealthController,
	jwtSecret string,
	blacklistRepo repository.TokenBlacklistRepository,
	rateLimiter middleware.RateLimiter,
) {
	if blacklistRepo == nil {
		blacklistRepo = repository.NewTokenBlacklistRepository(nil)
	}
	if rateLimiter == nil {
		rateLimiter = middleware.NewRateLimiter(nil)
	}

	// Global Middlewares
	app.Use(middleware.NewRecoverMiddleware())
	if app.Config().AppName != "url_shortener_benchmark" {
		app.Use(middleware.NewLoggerMiddleware())
	}
	app.Use(middleware.NewCORSMiddleware())
	app.Use(middleware.NewSecurityHeadersMiddleware())

	// Health Check
	app.Get("/health", healthController.HealthCheck)

	// API v1 routes
	api := app.Group("/api/v1")
	{
		// Auth routes
		auth := api.Group("/auth")
		auth.Post("/register", middleware.NewAuthRegisterRateLimiter(rateLimiter), authController.Register)
		auth.Post("/login", middleware.NewAuthLoginRateLimiter(rateLimiter), authController.Login)
		auth.Post("/logout", middleware.NewJWTMiddleware(jwtSecret, blacklistRepo), authController.Logout)
		auth.Get("/me", middleware.NewJWTMiddleware(jwtSecret, blacklistRepo), authController.GetProfile)

		// User dashboard routes (Protected)
		user := api.Group("/user", middleware.NewJWTMiddleware(jwtSecret, blacklistRepo))
		user.Get("/links", linkController.GetUserLinks)

		// Links routes (with optional JWT auth to identify user + rate limiting)
		links := api.Group("/links")
		links.Post("/", middleware.NewOptionalJWTMiddleware(jwtSecret, blacklistRepo), middleware.NewLinkCreationRateLimiter(rateLimiter), linkController.Create)
		links.Get("/:short_code/analytics", middleware.NewOptionalJWTMiddleware(jwtSecret, blacklistRepo), linkController.GetAnalytics)
	}

	// High-Throughput Redirection Route with DoS & Scan Rate Limiting
	app.Get("/:short_code", middleware.NewRedirectRateLimiter(rateLimiter), linkController.Redirect)
}
