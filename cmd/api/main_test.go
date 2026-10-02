package main_test

import (
	"bytes"
	"encoding/json"
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
		NodeID: 1,
	}

	val := validator.NewValidator()
	snowNode, _ := utils.NewSnowflakeNode(cfg.NodeID)
	// Passing nil pool and redis triggers in-memory fallback mode
	repo := repository.NewLinkRepository(nil, nil)
	svc := service.NewLinkService(cfg, repo, snowNode)
	linkCtrl := controller.NewLinkController(svc, val)
	healthCtrl := controller.NewHealthController()

	app := fiber.New(fiber.Config{
		ErrorHandler: exeption.ErrorHandler,
	})

	router.SetupRouter(app, linkCtrl, healthCtrl)
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

func TestCreateShortLinkAndRedirect(t *testing.T) {
	app := setupTestApp()

	// 1. Create Short Link
	createPayload := web.CreateLinkRequest{
		OriginalURL: "https://google.com",
		CustomAlias: "my-custom-test-link",
	}
	payloadBytes, _ := json.Marshal(createPayload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var res struct {
		Success bool             `json:"success"`
		Data    web.LinkResponse `json:"data"`
	}
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &res)
	assert.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, "my-custom-test-link", res.Data.ShortCode)
	assert.Equal(t, "https://google.com", res.Data.OriginalURL)

	// 2. Redirect
	redirectReq := httptest.NewRequest(http.MethodGet, "/my-custom-test-link", nil)
	redirectResp, err := app.Test(redirectReq)

	assert.NoError(t, err)
	// PRD requires HTTP 307 Temporary Redirect
	assert.Equal(t, http.StatusTemporaryRedirect, redirectResp.StatusCode)
	assert.Equal(t, "https://google.com", redirectResp.Header.Get("Location"))

	// 3. Analytics
	analyticsReq := httptest.NewRequest(http.MethodGet, "/api/v1/links/my-custom-test-link/analytics", nil)
	analyticsResp, err := app.Test(analyticsReq)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, analyticsResp.StatusCode)

	analyticsBody, _ := io.ReadAll(analyticsResp.Body)
	assert.Contains(t, string(analyticsBody), "my-custom-test-link")
}

func TestCreateShortLinkValidation(t *testing.T) {
	app := setupTestApp()

	// Empty URL should fail validation
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
