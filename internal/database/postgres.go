package database

import (
	"context"
	"fmt"
	"log"
	"time"
	"url_shortener/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPostgresPool initializes and returns a PostgreSQL connection pool using pgxpool.
func NewPostgresPool(cfg *config.Config) (*pgxpool.Pool, func(), error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse postgres config: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.Database.MaxConns)
	poolConfig.MinConns = int32(cfg.Database.MinConns)
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 15 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Printf("[WARN] Failed to connect to PostgreSQL: %v (running in disconnected mode)", err)
		return nil, func() {}, nil
	}

	if err := pool.Ping(ctx); err != nil {
		log.Printf("[WARN] PostgreSQL ping failed: %v (running in disconnected mode)", err)
		pool.Close()
		return nil, func() {}, nil
	}

	log.Println("[INFO] Successfully connected to PostgreSQL")

	cleanup := func() {
		if pool != nil {
			pool.Close()
			log.Println("[INFO] Closed PostgreSQL connection pool")
		}
	}

	return pool, cleanup, nil
}

