package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
	"url_shortener/internal/analytics"
	"url_shortener/internal/config"
	"url_shortener/internal/controller"
	"url_shortener/internal/exeption"
	"url_shortener/internal/middleware"
	"url_shortener/internal/model/web"
	"url_shortener/internal/repository"
	"url_shortener/internal/router"
	"url_shortener/internal/service"
	"url_shortener/pkg/utils"
	"url_shortener/pkg/validator"

	"github.com/gofiber/fiber/v3"
)

func setupBenchmarkApp(b *testing.B) (*fiber.App, analytics.WorkerPool, string) {
	cfg := &config.Config{
		App: config.AppConfig{
			Name:    "url_shortener_benchmark",
			Env:     "benchmark",
			Port:    "3000",
			BaseURL: "http://localhost:3000",
		},
		JWT: config.JWTConfig{
			Secret:        "benchmark-jwt-secret-key-32-bytes",
			ExpiresInHour: 24,
		},
		NodeID: 1,
	}

	val := validator.NewValidator()
	snowNode, _ := utils.NewSnowflakeNode(cfg.NodeID)
	linkRepo := repository.NewLinkRepository(nil, nil)
	userRepo := repository.NewUserRepository(nil)
	blacklistRepo := repository.NewTokenBlacklistRepository(nil)
	limiter := middleware.NewRateLimiter(nil)

	wp := analytics.NewWorkerPool(linkRepo, analytics.Config{
		BufferSize:    100000,
		BatchSize:     500,
		FlushInterval: 100 * time.Millisecond,
	})
	wp.Start()

	mockDNS := func(ctx context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}
	ssrfVal := validator.NewSSRFValidator(cfg.App.BaseURL, mockDNS)

	linkSvc := service.NewLinkService(cfg, linkRepo, snowNode, wp, ssrfVal)
	authSvc := service.NewAuthService(cfg, userRepo, blacklistRepo)

	linkCtrl := controller.NewLinkController(linkSvc, val)
	authCtrl := controller.NewAuthController(authSvc, val)
	healthCtrl := controller.NewHealthController()

	app := fiber.New(fiber.Config{
		AppName:      "url_shortener_benchmark",
		ErrorHandler: exeption.ErrorHandler,
	})

	router.SetupRouter(app, linkCtrl, authCtrl, healthCtrl, cfg.JWT.Secret, blacklistRepo, limiter)

	// Pre-seed a test link
	createPayload := web.CreateLinkRequest{
		OriginalURL: "https://example.com/target-benchmark-page",
	}
	res, err := linkSvc.CreateShortLink(context.Background(), &createPayload)
	if err != nil {
		b.Fatalf("failed to seed benchmark link: %v", err)
	}

	return app, wp, res.ShortCode
}

// BenchmarkRedirectThroughput benchmarks the read/redirect path under high concurrency.
// Target: >= 10,000 RPS.
func BenchmarkRedirectThroughput(b *testing.B) {
	app, wp, shortCode := setupBenchmarkApp(b)
	defer wp.Stop()

	targetPath := "/" + shortCode
	b.ResetTimer()

	var counter uint64
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c := atomic.AddUint64(&counter, 1)
			req := httptest.NewRequest(http.MethodGet, targetPath, nil)
			req.Header.Set("User-Agent", "benchmark-agent/1.0")
			// Vary IPs to simulate real traffic and distributed subnets
			req.Header.Set("X-Forwarded-For", fmt.Sprintf("103.%d.%d.1", (c/256)%250+1, c%250+1))

			resp, err := app.Test(req)
			if err != nil || resp.StatusCode != http.StatusTemporaryRedirect {
				b.Errorf("unexpected status: %v, err: %v", resp.StatusCode, err)
			}
		}
	})
}

// BenchmarkLinkCreationThroughput benchmarks the URL shortening path under high concurrency.
// Target: >= 1,000 RPS.
func BenchmarkLinkCreationThroughput(b *testing.B) {
	app, wp, _ := setupBenchmarkApp(b)
	defer wp.Stop()

	b.ResetTimer()

	var counter uint64
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c := atomic.AddUint64(&counter, 1)
			payload := web.CreateLinkRequest{
				OriginalURL: fmt.Sprintf("https://example.com/page/%d", c),
			}
			bodyBytes, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/links", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			// Vary IP to prevent anonymous rate limit on single IP
			req.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.%d.%d", (c/256)%250+1, c%250+1))

			resp, err := app.Test(req)
			if err != nil || resp.StatusCode != http.StatusCreated {
				b.Errorf("unexpected creation status: %v, err: %v", resp.StatusCode, err)
			}
		}
	})
}

// BenchmarkSlidingWindowRateLimiter benchmarks the in-memory/Redis rate limiter under extreme throughput.
func BenchmarkSlidingWindowRateLimiter(b *testing.B) {
	limiter := middleware.NewRateLimiter(nil)
	ctx := context.Background()

	b.ResetTimer()

	var counter uint64
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c := atomic.AddUint64(&counter, 1)
			key := fmt.Sprintf("key-%d", c%500)
			_, _ = limiter.Allow(ctx, key, 100000, time.Minute)
		}
	})
}
