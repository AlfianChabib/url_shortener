package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig
	Database DBConfig
	Redis    RedisConfig
	JWT      JWTConfig
	NodeID   int64
}

type AppConfig struct {
	Name    string
	Env     string
	Port    string
	BaseURL string
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
	MaxConns int
	MinConns int
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret        string
	ExpiresInHour int
}

// LoadConfig reads configuration from file or environment variables
func LoadConfig(path ...string) (*Config, error) {
	v := viper.New()

	v.SetDefault("APP_NAME", "url_shortener")
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("APP_PORT", "3000")
	v.SetDefault("APP_BASE_URL", "http://localhost:3000")

	v.SetDefault("DB_HOST", "localhost")
	v.SetDefault("DB_PORT", "5432")
	v.SetDefault("DB_USER", "postgres")
	v.SetDefault("DB_PASSWORD", "postgres")
	v.SetDefault("DB_NAME", "url_shortener_db")
	v.SetDefault("DB_SSL_MODE", "disable")
	v.SetDefault("DB_MAX_CONNS", 80)
	v.SetDefault("DB_MIN_CONNS", 40)

	v.SetDefault("REDIS_HOST", "localhost")
	v.SetDefault("REDIS_PORT", "6379")
	v.SetDefault("REDIS_PASSWORD", "")
	v.SetDefault("REDIS_DB", 0)

	v.SetDefault("JWT_SECRET", "super-secret-jwt-key-minimum-32-chars-long!")
	v.SetDefault("JWT_EXPIRES_IN_HOUR", 24)

	v.SetDefault("NODE_ID", 1)

	// Automatic environment variables
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Config file path if provided or default
	configPath := "."
	if len(path) > 0 && path[0] != "" {
		configPath = path[0]
	}

	v.AddConfigPath(configPath)
	v.SetConfigName(".env")
	v.SetConfigType("env")

	// If .env exists, read it
	_ = v.ReadInConfig()

	cfg := &Config{
		App: AppConfig{
			Name:    v.GetString("APP_NAME"),
			Env:     v.GetString("APP_ENV"),
			Port:    v.GetString("APP_PORT"),
			BaseURL: v.GetString("APP_BASE_URL"),
		},
		Database: DBConfig{
			Host:     v.GetString("DB_HOST"),
			Port:     v.GetString("DB_PORT"),
			User:     v.GetString("DB_USER"),
			Password: v.GetString("DB_PASSWORD"),
			Name:     v.GetString("DB_NAME"),
			SSLMode:  v.GetString("DB_SSL_MODE"),
			MaxConns: v.GetInt("DB_MAX_CONNS"),
			MinConns: v.GetInt("DB_MIN_CONNS"),
		},
		Redis: RedisConfig{
			Host:     v.GetString("REDIS_HOST"),
			Port:     v.GetString("REDIS_PORT"),
			Password: v.GetString("REDIS_PASSWORD"),
			DB:       v.GetInt("REDIS_DB"),
		},
		JWT: JWTConfig{
			Secret:        v.GetString("JWT_SECRET"),
			ExpiresInHour: v.GetInt("JWT_EXPIRES_IN_HOUR"),
		},
		NodeID: v.GetInt64("NODE_ID"),
	}

	return cfg, nil
}
