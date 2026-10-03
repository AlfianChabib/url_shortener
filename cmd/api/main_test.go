package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"url_shortener/internal/config"
	"url_shortener/internal/controller"
	"url_shortener/internal/exeption"
	"url_shortener/internal/model/web"
	"url_shortener/internal/repository"
	"url_shortener/internal/router"
	"url_shortener/internal/service"
	"url_shortener/pkg/utils"
	"url_shortener/pkg/validator"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
)

func setupTestApp() *fiber.App {
	cfg := &config.Config{
		App: config.AppConfig{
			Name:    "url_shortener_test",
			Env:     "test",
			Port:    "3000",
			BaseURL: "http://localhost:3000",
		},
		JWT: config.JWTConfig{
			Secret:        "test-jwt-secret-key-32-bytes-long",
			ExpiresInHour: 24,
		},
		NodeID: 1,
	}

	val := validator.NewValidator()
	snowNode, _ := utils.NewSnowflakeNode(cfg.NodeID)

	// Passing nil pool and redis triggers in-memory fallback mode
	linkRepo := repository.NewLinkRepository(nil, nil)
	userRepo := repository.NewUserRepository(nil)

	linkSvc := service.NewLinkService(cfg, linkRepo, snowNode)
	authSvc := service.NewAuthService(cfg, userRepo)

	linkCtrl := controller.NewLinkController(linkSvc, val)
	authCtrl := controller.NewAuthController(authSvc, val)
	healthCtrl := controller.NewHealthController()

	app := fiber.New(fiber.Config{
		ErrorHandler: exeption.ErrorHandler,
	})

	router.SetupRouter(app, linkCtrl, authCtrl, healthCtrl, cfg.JWT.Secret)
	return app
}

func TestHealthCheck(t *testing.T) {
	app := setupTestApp()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "healthy")
}

func TestAuthRegisterLoginAndProfile(t *testing.T) {
	app := setupTestApp()

	// 1. Register User
	registerPayload := web.RegisterRequest{
		Email:    "testuser@example.com",
		Username: "testuser",
		Password: "Password123!",
	}
	regBytes, _ := json.Marshal(registerPayload)
	regReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regBytes))
	regReq.Header.Set("Content-Type", "application/json")

	regResp, err := app.Test(regReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, regResp.StatusCode)

	// 2. Login User with Username
	loginPayload := web.LoginRequest{
		Identifier: "testuser",
		Password:   "Password123!",
	}
	loginBytes, _ := json.Marshal(loginPayload)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBytes))
	loginReq.Header.Set("Content-Type", "application/json")

	loginResp, err := app.Test(loginReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, loginResp.StatusCode)

	var loginResult struct {
		Success bool              `json:"success"`
		Data    web.LoginResponse `json:"data"`
	}
	body, _ := io.ReadAll(loginResp.Body)
	err = json.Unmarshal(body, &loginResult)
	assert.NoError(t, err)
	assert.NotEmpty(t, loginResult.Data.AccessToken)
	token := loginResult.Data.AccessToken

	// 3. Access Protected Profile with Bearer Token
	profileReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	profileReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	profileResp, err := app.Test(profileReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, profileResp.StatusCode)

	// 4. Access Protected Profile without Bearer Token (Should be 401 Unauthorized)
	unauthReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	unauthResp, err := app.Test(unauthReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, unauthResp.StatusCode)
}

func TestDualModeLinkCreation(t *testing.T) {
	app := setupTestApp()

	// 1. Register and Login to get token
	registerPayload := web.RegisterRequest{
		Email:    "creator@example.com",
		Username: "creator",
		Password: "Password123!",
	}
	regBytes, _ := json.Marshal(registerPayload)
	regReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regBytes))
	regReq.Header.Set("Content-Type", "application/json")
	_, _ = app.Test(regReq)

	loginPayload := web.LoginRequest{
		Identifier: "creator@example.com",
		Password:   "Password123!",
	}
	loginBytes, _ := json.Marshal(loginPayload)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBytes))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, _ := app.Test(loginReq)

	var loginResult struct {
		Data web.LoginResponse `json:"data"`
	}
	body, _ := io.ReadAll(loginResp.Body)
	_ = json.Unmarshal(body, &loginResult)
	token := loginResult.Data.AccessToken

	// 2. Authenticated user can create link with custom alias
	customPayload := web.CreateLinkRequest{
		OriginalURL: "https://example.com/custom",
		CustomAlias: "my-promo-alias",
	}
	cBytes, _ := json.Marshal(customPayload)
	cReq := httptest.NewRequest(http.MethodPost, "/api/v1/links", bytes.NewReader(cBytes))
	cReq.Header.Set("Content-Type", "application/json")
	cReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	cResp, err := app.Test(cReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, cResp.StatusCode)

	// 3. Guest (unauthenticated) trying to use custom alias MUST be rejected (401 Unauthorized)
	guestCustomReq := httptest.NewRequest(http.MethodPost, "/api/v1/links", bytes.NewReader(cBytes))
	guestCustomReq.Header.Set("Content-Type", "application/json")
	guestCustomResp, err := app.Test(guestCustomReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, guestCustomResp.StatusCode)

	// 4. Guest (unauthenticated) creating random link succeeds
	anonPayload := web.CreateLinkRequest{
		OriginalURL: "https://example.com/anonymous",
	}
	anonBytes, _ := json.Marshal(anonPayload)
	anonReq := httptest.NewRequest(http.MethodPost, "/api/v1/links", bytes.NewReader(anonBytes))
	anonReq.Header.Set("Content-Type", "application/json")
	anonResp, err := app.Test(anonReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, anonResp.StatusCode)
}

func TestRedirectAndAnalytics(t *testing.T) {
	app := setupTestApp()

	// Create short link (guest)
	anonPayload := web.CreateLinkRequest{
		OriginalURL: "https://google.com",
	}
	anonBytes, _ := json.Marshal(anonPayload)
	anonReq := httptest.NewRequest(http.MethodPost, "/api/v1/links", bytes.NewReader(anonBytes))
	anonReq.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(anonReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var res struct {
		Data web.LinkResponse `json:"data"`
	}
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &res)
	shortCode := res.Data.ShortCode

	// Redirect
	redirectReq := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	redirectResp, err := app.Test(redirectReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTemporaryRedirect, redirectResp.StatusCode)
	assert.Equal(t, "https://google.com", redirectResp.Header.Get("Location"))

	// Analytics
	analyticsReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/links/%s/analytics", shortCode), nil)
	analyticsResp, err := app.Test(analyticsReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, analyticsResp.StatusCode)
}

func TestCreateShortLinkValidation(t *testing.T) {
	app := setupTestApp()

	createPayload := web.CreateLinkRequest{
		OriginalURL: "",
	}
	payloadBytes, _ := json.Marshal(createPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRedirectNotFound(t *testing.T) {
	app := setupTestApp()

	req := httptest.NewRequest(http.MethodGet, "/non-existent-code-12345", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
