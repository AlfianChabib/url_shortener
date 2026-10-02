package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"url_shortener/internal/config"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	app, cleanup, err := InitializeServer()
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}
	defer cleanup()

	port := cfg.App.Port
	if port == "" {
		port = "3000"
	}

	// Channel to listen for OS signals for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Starting %s server on port %s (%s)...", cfg.App.Name, port, cfg.App.Env)
		if err := app.Listen(":" + port); err != nil {
			log.Printf("Server stopped: %v", err)
		}
	}()

	<-quit
	log.Println("Shutting down server gracefully...")

	if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}

	log.Println("Server gracefully stopped.")
}

