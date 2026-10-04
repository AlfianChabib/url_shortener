package service_test

import (
	"context"
	"testing"
	"url_shortener/internal/config"
	"url_shortener/internal/model/web"
	"url_shortener/internal/repository"
	"url_shortener/internal/service"

	"github.com/stretchr/testify/assert"
)

func TestAuthService(t *testing.T) {
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:        "test-jwt-secret-key-32-bytes-long",
			ExpiresInHour: 24,
		},
	}
	repo := repository.NewUserRepository(nil)
	blacklistRepo := repository.NewTokenBlacklistRepository(nil)
	svc := service.NewAuthService(cfg, repo, blacklistRepo)
	ctx := context.Background()

	// 1. Register User
	registerReq := &web.RegisterRequest{
		Email:    "test@example.com",
		Username: "testuser",
		Password: "Password123!",
	}
	userResp, err := svc.Register(ctx, registerReq)
	assert.NoError(t, err)
	assert.Equal(t, "test@example.com", userResp.Email)
	assert.Equal(t, "testuser", userResp.Username)

	// Duplicate registration should fail
	_, err = svc.Register(ctx, registerReq)
	assert.Error(t, err)

	// 2. Login with email
	loginWithEmail := &web.LoginRequest{
		Identifier: "test@example.com",
		Password:   "Password123!",
	}
	loginResp, err := svc.Login(ctx, loginWithEmail)
	assert.NoError(t, err)
	assert.NotEmpty(t, loginResp.AccessToken)
	assert.Equal(t, "Bearer", loginResp.TokenType)
	assert.Equal(t, userResp.ID, loginResp.User.ID)

	// 3. Login with username
	loginWithUsername := &web.LoginRequest{
		Identifier: "testuser",
		Password:   "Password123!",
	}
	loginResp2, err := svc.Login(ctx, loginWithUsername)
	assert.NoError(t, err)
	assert.NotEmpty(t, loginResp2.AccessToken)

	// 4. Login with invalid password
	loginBadPass := &web.LoginRequest{
		Identifier: "testuser",
		Password:   "WrongPassword!",
	}
	_, err = svc.Login(ctx, loginBadPass)
	assert.Error(t, err)

	// 5. GetProfile
	profile, err := svc.GetProfile(ctx, userResp.ID)
	assert.NoError(t, err)
	assert.Equal(t, userResp.ID, profile.ID)
	assert.Equal(t, userResp.Email, profile.Email)

	// 6. Logout and verify token is blacklisted
	err = svc.Logout(ctx, loginResp.AccessToken)
	assert.NoError(t, err)

	isBlacklisted, err := blacklistRepo.IsTokenBlacklisted(ctx, loginResp.AccessToken)
	assert.NoError(t, err)
	assert.True(t, isBlacklisted)
}
