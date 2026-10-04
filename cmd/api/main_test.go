package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"url_shortener/internal/analytics"
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

func setupTestApp() (*fiber.App, analytics.WorkerPool) {
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

	// Instant flush worker pool for tests
	wp := analytics.NewWorkerPool(linkRepo, analytics.Config{
		BufferSize:    1000,
		BatchSize:     1,
		FlushInterval: 20 * time.Millisecond,
	})
	wp.Start()

	linkSvc := service.NewLinkService(cfg, linkRepo, snowNode, wp)
	authSvc := service.NewAuthService(cfg, userRepo)

	linkCtrl := controller.NewLinkController(linkSvc, val)
	authCtrl := controller.NewAuthController(authSvc, val)
	healthCtrl := controller.NewHealthController()

	app := fiber.New(fiber.Config{
		ErrorHandler: exeption.ErrorHandler,
	})

	router.SetupRouter(app, linkCtrl, authCtrl, healthCtrl, cfg.JWT.Secret)
	return app, wp
}

func TestHealthCheck(t *testing.T) {
	app, wp := setupTestApp()
	defer wp.Stop()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "healthy")
}

func TestAuthRegisterLoginAndProfile(t *testing.T) {
	app, wp := setupTestApp()
	defer wp.Stop()

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
		Data web.LoginResponse `json:"data"`
	}
	loginBody, _ := io.ReadAll(loginResp.Body)
	err = json.Unmarshal(loginBody, &loginResult)
	assert.NoError(t, err)
	assert.NotEmpty(t, loginResult.Data.AccessToken)

	// 3. Get Profile (/api/v1/auth/me) with JWT
	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginResult.Data.AccessToken)

	meResp, err := app.Test(meReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, meResp.StatusCode)

	var meResult struct {
		Data web.UserResponse `json:"data"`
	}
	meBody, _ := io.ReadAll(meResp.Body)
	_ = json.Unmarshal(meBody, &meResult)
	assert.Equal(t, "testuser", meResult.Data.Username)
	assert.Equal(t, "testuser@example.com", meResult.Data.Email)

	// 4. Access /api/v1/auth/me without JWT fails (401)
	unauthReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	unauthResp, err := app.Test(unauthReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, unauthResp.StatusCode)
}

func TestDualModeLinkCreation(t *testing.T) {
	app, wp := setupTestApp()
	defer wp.Stop()

	// 1. Setup user
	regPayload := web.RegisterRequest{
		Email:    "aliasuser@example.com",
		Username: "aliasuser",
		Password: "Password123!",
	}
	b, _ := json.Marshal(regPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	_, _ = app.Test(req)

	loginPayload := web.LoginRequest{
		Identifier: "aliasuser",
		Password:   "Password123!",
	}
	b, _ = json.Marshal(loginPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	loginResp, _ := app.Test(req)

	var loginResult struct {
		Data web.LoginResponse `json:"data"`
	}
	loginBody, _ := io.ReadAll(loginResp.Body)
	_ = json.Unmarshal(loginBody, &loginResult)
	token := loginResult.Data.AccessToken

	// 2. Authenticated user creating custom alias succeeds
	authAliasPayload := web.CreateLinkRequest{
		OriginalURL: "https://example.com/custom-campaign",
		CustomAlias: "my-custom-promo",
	}
	b, _ = json.Marshal(authAliasPayload)
	authReq := httptest.NewRequest(http.MethodPost, "/api/v1/links", bytes.NewReader(b))
	authReq.Header.Set("Content-Type", "application/json")
	authReq.Header.Set("Authorization", "Bearer "+token)

	authResp, err := app.Test(authReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, authResp.StatusCode)

	var linkResult struct {
		Data web.LinkResponse `json:"data"`
	}
	linkBody, _ := io.ReadAll(authResp.Body)
	_ = json.Unmarshal(linkBody, &linkResult)
	assert.Equal(t, "my-custom-promo", linkResult.Data.ShortCode)

	// 3. Guest (unauthenticated) creating custom alias FAILS (401)
	guestAliasPayload := web.CreateLinkRequest{
		OriginalURL: "https://example.com/custom-campaign",
		CustomAlias: "guest-not-allowed",
	}
	b, _ = json.Marshal(guestAliasPayload)
	guestReq := httptest.NewRequest(http.MethodPost, "/api/v1/links", bytes.NewReader(b))
	guestReq.Header.Set("Content-Type", "application/json")

	guestResp, err := app.Test(guestReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, guestResp.StatusCode)

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
	app, wp := setupTestApp()
	defer wp.Stop()

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

	// Redirect 1: Mobile iOS from Indonesia
	redirectReq1 := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	redirectReq1.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Mobile Safari/604.1")
	redirectReq1.Header.Set("X-Forwarded-For", "203.0.113.1")
	redirectReq1.Header.Set("CF-IPCountry", "ID")
	redirectResp1, err := app.Test(redirectReq1)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTemporaryRedirect, redirectResp1.StatusCode)
	assert.Equal(t, "https://google.com", redirectResp1.Header.Get("Location"))

	// Redirect 2: Desktop Windows from Singapore
	redirectReq2 := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	redirectReq2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36")
	redirectReq2.Header.Set("X-Forwarded-For", "203.0.113.2")
	redirectReq2.Header.Set("CF-IPCountry", "SG")
	redirectResp2, err := app.Test(redirectReq2)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTemporaryRedirect, redirectResp2.StatusCode)

	// Redirect 3: Bot
	redirectReq3 := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	redirectReq3.Header.Set("User-Agent", "Googlebot/2.1 (+http://www.google.com/bot.html)")
	redirectReq3.Header.Set("X-Forwarded-For", "203.0.113.3")
	redirectReq3.Header.Set("CF-IPCountry", "US")
	redirectResp3, err := app.Test(redirectReq3)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTemporaryRedirect, redirectResp3.StatusCode)

	// Flush worker pool to ensure batch has been recorded
	wp.Flush()

	// Query Analytics endpoint
	analyticsReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/links/%s/analytics", shortCode), nil)
	analyticsResp, err := app.Test(analyticsReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, analyticsResp.StatusCode)

	var analyticsResult struct {
		Data web.AnalyticsResponse `json:"data"`
	}
	analyticsBody, _ := io.ReadAll(analyticsResp.Body)
	err = json.Unmarshal(analyticsBody, &analyticsResult)
	assert.NoError(t, err)

	data := analyticsResult.Data
	assert.Equal(t, shortCode, data.ShortCode)
	assert.Equal(t, int64(3), data.TotalClicks)
	assert.Equal(t, int64(3), data.UniqueClicks)

	// Verify device stats are non-hardcoded and dynamic
	assert.InDelta(t, 33.3, data.Devices.Mobile, 0.5)
	assert.InDelta(t, 33.3, data.Devices.Desktop, 0.5)
	assert.InDelta(t, 33.3, data.Devices.Bot, 0.5)

	// Verify top countries
	assert.NotEmpty(t, data.TopCountries)
	assert.NotEmpty(t, data.TimeSeries)
}

func TestUserLinksDashboard(t *testing.T) {
	app, wp := setupTestApp()
	defer wp.Stop()

	// 1. Register and login a user
	regPayload := web.RegisterRequest{
		Email:    "dashboarduser@example.com",
		Username: "dashboarduser",
		Password: "Password123!",
	}
	b, _ := json.Marshal(regPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	_, _ = app.Test(req)

	loginPayload := web.LoginRequest{
		Identifier: "dashboarduser",
		Password:   "Password123!",
	}
	b, _ = json.Marshal(loginPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	loginResp, _ := app.Test(req)

	var loginResult struct {
		Data web.LoginResponse `json:"data"`
	}
	loginBody, _ := io.ReadAll(loginResp.Body)
	_ = json.Unmarshal(loginBody, &loginResult)
	token := loginResult.Data.AccessToken

	// 2. Unauthenticated request to GET /api/v1/user/links returns 401
	unauthReq := httptest.NewRequest(http.MethodGet, "/api/v1/user/links", nil)
	unauthResp, err := app.Test(unauthReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, unauthResp.StatusCode)

	// 3. User initially has 0 links
	dashReq := httptest.NewRequest(http.MethodGet, "/api/v1/user/links", nil)
	dashReq.Header.Set("Authorization", "Bearer "+token)
	dashResp, err := app.Test(dashReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, dashResp.StatusCode)

	var emptyDash struct {
		Data web.UserLinksResponse `json:"data"`
	}
	dashBody, _ := io.ReadAll(dashResp.Body)
	_ = json.Unmarshal(dashBody, &emptyDash)
	assert.Equal(t, int64(0), emptyDash.Data.Total)
	assert.Empty(t, emptyDash.Data.Links)

	// 4. Create 2 links as this user
	link1 := web.CreateLinkRequest{
		OriginalURL: "https://example.com/link-1",
		CustomAlias: "user-link-one",
	}
	b, _ = json.Marshal(link1)
	createReq1 := httptest.NewRequest(http.MethodPost, "/api/v1/links", bytes.NewReader(b))
	createReq1.Header.Set("Content-Type", "application/json")
	createReq1.Header.Set("Authorization", "Bearer "+token)
	resp1, _ := app.Test(createReq1)
	assert.Equal(t, http.StatusCreated, resp1.StatusCode)

	link2 := web.CreateLinkRequest{
		OriginalURL: "https://example.com/link-2",
		CustomAlias: "user-link-two",
	}
	b, _ = json.Marshal(link2)
	createReq2 := httptest.NewRequest(http.MethodPost, "/api/v1/links", bytes.NewReader(b))
	createReq2.Header.Set("Content-Type", "application/json")
	createReq2.Header.Set("Authorization", "Bearer "+token)
	resp2, _ := app.Test(createReq2)
	assert.Equal(t, http.StatusCreated, resp2.StatusCode)

	// 5. Query user dashboard with pagination
	dashReq2 := httptest.NewRequest(http.MethodGet, "/api/v1/user/links?page=1&limit=10", nil)
	dashReq2.Header.Set("Authorization", "Bearer "+token)
	dashResp2, err := app.Test(dashReq2)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, dashResp2.StatusCode)

	var userLinks struct {
		Data web.UserLinksResponse `json:"data"`
	}
	dashBody2, _ := io.ReadAll(dashResp2.Body)
	_ = json.Unmarshal(dashBody2, &userLinks)
	assert.Equal(t, int64(2), userLinks.Data.Total)
	assert.Len(t, userLinks.Data.Links, 2)
	assert.Equal(t, 1, userLinks.Data.Page)
	assert.Equal(t, 10, userLinks.Data.Limit)
	assert.NotEmpty(t, userLinks.Data.Links[0].ShortURL)
	assert.NotEmpty(t, userLinks.Data.Links[0].OriginalURL)

	// 6. Test pagination limit=1
	pageReq := httptest.NewRequest(http.MethodGet, "/api/v1/user/links?page=2&limit=1", nil)
	pageReq.Header.Set("Authorization", "Bearer "+token)
	pageResp, err := app.Test(pageReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, pageResp.StatusCode)

	var pagedLinks struct {
		Data web.UserLinksResponse `json:"data"`
	}
	pageBody, _ := io.ReadAll(pageResp.Body)
	_ = json.Unmarshal(pageBody, &pagedLinks)
	assert.Equal(t, int64(2), pagedLinks.Data.Total)
	assert.Len(t, pagedLinks.Data.Links, 1)
	assert.Equal(t, 2, pagedLinks.Data.Page)
	assert.Equal(t, 1, pagedLinks.Data.Limit)
}

func TestCreateShortLinkValidation(t *testing.T) {
	app, wp := setupTestApp()
	defer wp.Stop()

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
	app, wp := setupTestApp()
	defer wp.Stop()

	req := httptest.NewRequest(http.MethodGet, "/non-existent-code-12345", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
