package repository_test

import (
	"context"
	"testing"
	"time"
	"url_shortener/internal/repository"

	"github.com/stretchr/testify/assert"
)

func TestTokenBlacklistRepository_InMemory(t *testing.T) {
	repo := repository.NewTokenBlacklistRepository(nil)
	ctx := context.Background()

	token := "sample-jwt-token-string"

	// 1. Initial check: not blacklisted
	blacklisted, err := repo.IsTokenBlacklisted(ctx, token)
	assert.NoError(t, err)
	assert.False(t, blacklisted)

	// 2. Blacklist with 500ms TTL
	err = repo.BlacklistToken(ctx, token, 500*time.Millisecond)
	assert.NoError(t, err)

	// 3. Immediately check: must be blacklisted
	blacklisted, err = repo.IsTokenBlacklisted(ctx, token)
	assert.NoError(t, err)
	assert.True(t, blacklisted)

	// 4. Other token: must NOT be blacklisted
	otherBlacklisted, err := repo.IsTokenBlacklisted(ctx, "different-token")
	assert.NoError(t, err)
	assert.False(t, otherBlacklisted)

	// 5. Wait for TTL to expire
	time.Sleep(550 * time.Millisecond)
	blacklisted, err = repo.IsTokenBlacklisted(ctx, token)
	assert.NoError(t, err)
	assert.False(t, blacklisted)
}
