package database

import (
	"testing"
	"url_shortener/internal/config"
)

func TestPostgresConnection(t *testing.T) {
	cfg, err := config.LoadConfig("../../")
	if err != nil {
		t.Skip("skipping, cannot load config")
	}

	db, cleanup, err := NewGormDB(cfg)
	if err != nil || db == nil {
		t.Skip("skipping, postgres not available")
	}
	defer cleanup()

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("failed to ping postgres: %v", err)
	}
}
