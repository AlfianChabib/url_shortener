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

	link := &domain.Link{
		ID:          id,
		ShortCode:   "repo-test",
		OriginalURL: "https://example.com/repo-test",
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
}
