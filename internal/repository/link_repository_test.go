package repository_test

import (
	"context"
	"testing"
	"time"
	"url_shortener/internal/model/domain"
	"url_shortener/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestLinkRepository(t *testing.T) {
	repo := repository.NewLinkRepository(nil, nil)
	ctx := context.Background()

	now := time.Now()
	id, err := uuid.NewV7()
	assert.NoError(t, err)

	userID, err := uuid.NewV7()
	assert.NoError(t, err)

	link := &domain.Link{
		ID:          id,
		ShortCode:   "repo-test",
		OriginalURL: "https://example.com/repo-test",
		UserID:      &userID,
		IsActive:    true,
		CreatedAt:   now,
	}

	// 1. Create Link
	err = repo.Create(ctx, link)
	assert.NoError(t, err)

	// Duplicate create should return error
	err = repo.Create(ctx, link)
	assert.Error(t, err)

	// 2. FindByShortCode
	found, err := repo.FindByShortCode(ctx, "repo-test")
	assert.NoError(t, err)
	assert.Equal(t, link.ID, found.ID)
	assert.Equal(t, link.ShortCode, found.ShortCode)
	assert.Equal(t, link.OriginalURL, found.OriginalURL)

	// Non-existent link
	notFound, err := repo.FindByShortCode(ctx, "not-exist")
	assert.Error(t, err)
	assert.Nil(t, notFound)

	// 3. Cache operations
	err = repo.SetCache(ctx, "repo-test", "https://example.com/repo-test", 10*time.Minute)
	assert.NoError(t, err)

	cachedURL, err := repo.GetCache(ctx, "repo-test")
	assert.NoError(t, err)
	assert.Equal(t, "https://example.com/repo-test", cachedURL)

	// 4. Click counting operations
	err = repo.IncrementClick(ctx, "repo-test")
	assert.NoError(t, err)
	err = repo.IncrementClick(ctx, "repo-test")
	assert.NoError(t, err)

	clicks, err := repo.GetClickCount(ctx, "repo-test")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), clicks)

	// 5. FindByUserID
	userLinks, total, err := repo.FindByUserID(ctx, userID, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, userLinks, 1)
	assert.Equal(t, "repo-test", userLinks[0].ShortCode)

	// 6. RecordClickEvents & GetAnalytics
	events := []*domain.ClickEvent{
		{
			ShortCode:   "repo-test",
			ClickedAt:   now,
			IPHash:      "hash-1",
			CountryCode: "ID",
			DeviceType:  "mobile",
			Browser:     "Safari",
			OS:          "iOS",
		},
		{
			ShortCode:   "repo-test",
			ClickedAt:   now,
			IPHash:      "hash-2",
			CountryCode: "ID",
			DeviceType:  "desktop",
			Browser:     "Chrome",
			OS:          "Windows",
		},
		{
			ShortCode:   "repo-test",
			ClickedAt:   now,
			IPHash:      "hash-1", // duplicate IP
			CountryCode: "SG",
			DeviceType:  "bot",
			Browser:     "Googlebot",
			OS:          "Bot",
		},
	}

	err = repo.RecordClickEvents(ctx, events)
	assert.NoError(t, err)

	analytics, err := repo.GetAnalytics(ctx, "repo-test")
	assert.NoError(t, err)
	assert.Equal(t, "repo-test", analytics.ShortCode)
	assert.Equal(t, int64(3), analytics.TotalClicks)
	assert.Equal(t, int64(2), analytics.UniqueClicks)
	assert.NotEmpty(t, analytics.TopCountries)
	assert.Equal(t, "ID", analytics.TopCountries[0].Code)
	assert.Equal(t, int64(2), analytics.TopCountries[0].Clicks)
	assert.InDelta(t, 33.3, analytics.Devices.Mobile, 0.1)
	assert.InDelta(t, 33.3, analytics.Devices.Desktop, 0.1)
	assert.InDelta(t, 33.3, analytics.Devices.Bot, 0.1)
	assert.NotEmpty(t, analytics.TimeSeries)
}
