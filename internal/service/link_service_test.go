package service_test

import (
	"context"
	"net"
	"testing"
	"time"
	"url_shortener/internal/analytics"
	"url_shortener/internal/config"
	"url_shortener/internal/model/web"
	"url_shortener/internal/repository"
	"url_shortener/internal/service"
	"url_shortener/pkg/utils"
	"url_shortener/pkg/validator"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestLinkService(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			BaseURL: "https://s.id",
		},
		JWT: config.JWTConfig{
			Secret: "test-salt-secret",
		},
		NodeID: 1,
	}

	repo := repository.NewLinkRepository(nil, nil)
	snowNode, _ := utils.NewSnowflakeNode(cfg.NodeID)
	wp := analytics.NewWorkerPool(repo, analytics.Config{
		BufferSize:    100,
		BatchSize:     1,
		FlushInterval: 10 * time.Millisecond,
	})
	wp.Start()
	defer wp.Stop()

	mockDNS := func(ctx context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}
	ssrfVal := validator.NewSSRFValidator(cfg.App.BaseURL, mockDNS)

	svc := service.NewLinkService(cfg, repo, snowNode, wp, ssrfVal)
	ctx := context.Background()

	userID, err := uuid.NewV7()
	assert.NoError(t, err)

	// 1. Create short link as authenticated user with custom alias
	aliasReq := &web.CreateLinkRequest{
		OriginalURL:    "https://example.com/promo",
		CustomAlias:    "promo-sale-2026",
		ExpiresInHours: 24,
	}
	res, err := svc.CreateShortLink(ctx, aliasReq, &userID)
	assert.NoError(t, err)
	assert.Equal(t, "promo-sale-2026", res.ShortCode)
	assert.Equal(t, "https://s.id/promo-sale-2026", res.ShortURL)

	// 1b. SSRF rejection check: private IP target must be blocked
	badReq := &web.CreateLinkRequest{
		OriginalURL: "http://127.0.0.1:8080/admin",
	}
	_, err = svc.CreateShortLink(ctx, badReq, &userID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SSRF protection")

	// 2. Guest cannot create custom alias
	_, err = svc.CreateShortLink(ctx, aliasReq)
	assert.Error(t, err)

	// 3. GetOriginalURL
	url, err := svc.GetOriginalURL(ctx, "promo-sale-2026")
	assert.NoError(t, err)
	assert.Equal(t, "https://example.com/promo", url)

	// 4. TrackClick telemetry
	svc.TrackClick("promo-sale-2026", "203.0.113.1", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Mobile Safari/604.1", "https://twitter.com", "ID")
	svc.TrackClick("promo-sale-2026", "203.0.113.2", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0", "https://linkedin.com", "SG")

	wp.Flush()

	// 5. GetAnalytics
	analyticsRes, err := svc.GetAnalytics(ctx, "promo-sale-2026")
	assert.NoError(t, err)
	assert.Equal(t, "promo-sale-2026", analyticsRes.ShortCode)
	assert.Equal(t, int64(2), analyticsRes.TotalClicks)
	assert.Equal(t, int64(2), analyticsRes.UniqueClicks)
	assert.Equal(t, 50.0, analyticsRes.Devices.Mobile)
	assert.Equal(t, 50.0, analyticsRes.Devices.Desktop)
	assert.NotEmpty(t, analyticsRes.TopCountries)

	// 6. GetUserLinks
	userLinks, err := svc.GetUserLinks(ctx, userID, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), userLinks.Total)
	assert.Len(t, userLinks.Links, 1)
	assert.Equal(t, "promo-sale-2026", userLinks.Links[0].ShortCode)
}
