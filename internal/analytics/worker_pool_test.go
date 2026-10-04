package analytics_test

import (
	"context"
	"testing"
	"time"
	"url_shortener/internal/analytics"
	"url_shortener/internal/model/domain"
	"url_shortener/internal/repository"

	"github.com/stretchr/testify/assert"
)

func TestWorkerPool_BatchAndFlush(t *testing.T) {
	repo := repository.NewLinkRepository(nil, nil)
	ctx := context.Background()

	// Use small batch and fast interval for testing
	cfg := analytics.Config{
		BufferSize:    100,
		BatchSize:     5,
		FlushInterval: 100 * time.Millisecond,
	}

	wp := analytics.NewWorkerPool(repo, cfg)
	wp.Start()
	defer wp.Stop()

	// Enqueue 3 events (less than BatchSize of 5)
	for i := 0; i < 3; i++ {
		ok := wp.Enqueue(&domain.ClickEvent{
			ShortCode:   "test-code",
			ClickedAt:   time.Now(),
			IPHash:      "hash-123",
			CountryCode: "ID",
			DeviceType:  "mobile",
		})
		assert.True(t, ok)
	}

	// Trigger immediate flush
	wp.Flush()

	analyticsData, err := repo.GetAnalytics(ctx, "test-code")
	assert.NoError(t, err)
	assert.Equal(t, int64(3), analyticsData.TotalClicks)
	assert.Equal(t, int64(1), analyticsData.UniqueClicks)
	assert.Equal(t, 100.0, analyticsData.Devices.Mobile)
}

func TestWorkerPool_GracefulStop(t *testing.T) {
	repo := repository.NewLinkRepository(nil, nil)
	ctx := context.Background()

	cfg := analytics.Config{
		BufferSize:    100,
		BatchSize:     50,
		FlushInterval: 10 * time.Second, // Long interval
	}

	wp := analytics.NewWorkerPool(repo, cfg)
	wp.Start()

	// Enqueue 4 events
	for i := 0; i < 4; i++ {
		wp.Enqueue(&domain.ClickEvent{
			ShortCode:   "stop-test",
			ClickedAt:   time.Now(),
			IPHash:      "hash-xyz",
			CountryCode: "SG",
			DeviceType:  "desktop",
		})
	}

	// Stop should flush all 4 events before terminating
	wp.Stop()

	analyticsData, err := repo.GetAnalytics(ctx, "stop-test")
	assert.NoError(t, err)
	assert.Equal(t, int64(4), analyticsData.TotalClicks)
	assert.Equal(t, 100.0, analyticsData.Devices.Desktop)
}
