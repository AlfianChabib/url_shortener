package database

import (
	"context"
	"fmt"
	"log"
	"time"
	"url_shortener/internal/config"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient initializes and returns a Redis client.
func NewRedisClient(cfg *config.Config) (*redis.Client, func(), error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("[WARN] Failed to connect to Redis: %v (running in disconnected mode)", err)
		return nil, func() {}, nil
	}

	log.Println("[INFO] Successfully connected to Redis")

	cleanup := func() {
		if client != nil {
			if err := client.Close(); err != nil {
				log.Printf("[ERROR] Error closing Redis client: %v", err)
			} else {
				log.Println("[INFO] Closed Redis connection")
			}
		}
	}

	return client, cleanup, nil
}

