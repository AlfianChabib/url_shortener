//go:build wireinject
// +build wireinject

package main

import (
	"url_shortener/internal/config"
	"url_shortener/internal/controller"
	"url_shortener/internal/database"
	"url_shortener/internal/exeption"
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

func provideFiberApp(
	linkCtrl controller.LinkController,
	healthCtrl controller.HealthController,
) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "High-Performance URL Shortener",
		ErrorHandler: exeption.ErrorHandler,
	})

	router.SetupRouter(app, linkCtrl, healthCtrl)

	return app
}

var serverSet = wire.NewSet(
	provideConfig,
	database.NewGormDB,
	database.NewRedisClient,
	provideSnowflakeNode,
	validator.NewValidator,
	repository.NewLinkRepository,
	service.NewLinkService,
	controller.NewLinkController,
	controller.NewHealthController,
	provideFiberApp,
)

// InitializeServer initializes the Fiber application with all dependencies injected.
func InitializeServer() (*fiber.App, func(), error) {
	wire.Build(serverSet)
	return nil, nil, nil
}
