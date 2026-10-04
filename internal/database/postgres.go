package database

import (
	"fmt"
	"log"
	"time"
	"url_shortener/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewGormDB initializes and returns a GORM DB instance configured for PostgreSQL.
func NewGormDB(cfg *config.Config) (*gorm.DB, func(), error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
		cfg.Database.Host,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.Port,
		cfg.Database.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Printf("[WARN] Failed to connect to PostgreSQL via GORM: %v (running in disconnected mode)", err)
		return nil, func() {}, nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("[WARN] Failed to get generic database object from GORM: %v", err)
		return nil, func() {}, nil
	}

	if err := sqlDB.Ping(); err != nil {
		log.Printf("[WARN] PostgreSQL ping failed via GORM: %v (running in disconnected mode)", err)
		sqlDB.Close()
		return nil, func() {}, nil
	}

	sqlDB.SetMaxOpenConns(cfg.Database.MaxConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MinConns)
	sqlDB.SetConnMaxLifetime(1 * time.Hour)
	sqlDB.SetConnMaxIdleTime(15 * time.Minute)

	log.Println("[INFO] Successfully connected to PostgreSQL via GORM")

	cleanup := func() {
		if sqlDB != nil {
			if err := sqlDB.Close(); err != nil {
				log.Printf("[ERROR] Error closing PostgreSQL connection: %v", err)
			} else {
				log.Println("[INFO] Closed PostgreSQL connection pool")
			}
		}
	}

	return db, cleanup, nil
}
