//go:build wireinject
// +build wireinject

package main

import (
	"url_shortener/internal/analytics"
	"url_shortener/internal/config"
	"url_shortener/internal/controller"
	"url_shortener/internal/database"
	"url_shortener/internal/exeption"
	"url_shortener/internal/middleware"
	"url_shortener/internal/repository"
	"url_shortener/internal/router"
	"url_shortener/internal/service"
	"url_shortener/pkg/utils"
	"url_shortener/pkg/validator"

	"github.com/bwmarrin/snowflake"
	"github.com/gofiber/fiber/v3"
	"github.com/google/wire"
)

func provideSnowflakeNode(cfg *config.Config) (*snowflake.Node, error) {
	return utils.NewSnowflakeNode(cfg.NodeID)
}

func provideConfig() (*config.Config, error) {
	return config.LoadConfig()
}

func provideSSRFValidator(cfg *config.Config) validator.SSRFValidator {
	return validator.NewSSRFValidator(cfg.App.BaseURL, nil)
}

func provideWorkerPool(repo repository.LinkRepository) (analytics.WorkerPool, func(), error) {
	wp := analytics.NewWorkerPool(repo, analytics.DefaultConfig())
	wp.Start()
	cleanup := func() {
		wp.Stop()
	}
	return wp, cleanup, nil
}

func provideFiberApp(
	cfg *config.Config,
	linkCtrl controller.LinkController,
	authCtrl controller.AuthController,
	healthCtrl controller.HealthController,
	blacklistRepo repository.TokenBlacklistRepository,
	rateLimiter middleware.RateLimiter,
) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "High-Performance URL Shortener",
		ErrorHandler: exeption.ErrorHandler,
	})

	router.SetupRouter(app, linkCtrl, authCtrl, healthCtrl, cfg.JWT.Secret, blacklistRepo, rateLimiter)

	return app
}

var serverSet = wire.NewSet(
	provideConfig,
	database.NewGormDB,
	database.NewRedisClient,
	provideSnowflakeNode,
	validator.NewValidator,
	provideSSRFValidator,
	repository.NewTokenBlacklistRepository,
	middleware.NewRateLimiter,
	repository.NewLinkRepository,
	repository.NewUserRepository,
	provideWorkerPool,
	service.NewLinkService,
	service.NewAuthService,
	controller.NewLinkController,
	controller.NewAuthController,
	controller.NewHealthController,
	provideFiberApp,
)

// InitializeServer initializes the Fiber application with all dependencies injected.
func InitializeServer() (*fiber.App, func(), error) {
	wire.Build(serverSet)
	return nil, nil, nil
}
