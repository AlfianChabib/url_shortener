package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenBlacklistRepository manages revocation and querying of invalidated JWT tokens.
type TokenBlacklistRepository interface {
	BlacklistToken(ctx context.Context, token string, ttl time.Duration) error
	IsTokenBlacklisted(ctx context.Context, token string) (bool, error)
	HashToken(token string) string
}

type tokenBlacklistRepoImpl struct {
	redis *redis.Client

	// In-memory fallback store when Redis is unavailable
	mu       sync.RWMutex
	memStore map[string]time.Time
}

// NewTokenBlacklistRepository creates a new instance of TokenBlacklistRepository.
func NewTokenBlacklistRepository(redisClient *redis.Client) TokenBlacklistRepository {
	return &tokenBlacklistRepoImpl{
		redis:    redisClient,
		memStore: make(map[string]time.Time),
	}
}

// HashToken computes a SHA-256 hex digest of the raw JWT token string.
func (r *tokenBlacklistRepoImpl) HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// BlacklistToken saves the hashed token into Redis or in-memory fallback with the given TTL.
func (r *tokenBlacklistRepoImpl) BlacklistToken(ctx context.Context, token string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}

	tokenHash := r.HashToken(token)
	key := "blacklist:jwt:" + tokenHash

	if r.redis != nil {
		if err := r.redis.Set(ctx, key, "revoked", ttl).Err(); err == nil {
			return nil
		}
	}

	// In-memory fallback
	r.mu.Lock()
	defer r.mu.Unlock()
	r.memStore[tokenHash] = time.Now().Add(ttl)

	// Clean up expired tokens lazily
	now := time.Now()
	for k, exp := range r.memStore {
		if now.After(exp) {
			delete(r.memStore, k)
		}
	}

	return nil
}

// IsTokenBlacklisted checks if the token has been revoked.
func (r *tokenBlacklistRepoImpl) IsTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	tokenHash := r.HashToken(token)
	key := "blacklist:jwt:" + tokenHash

	if r.redis != nil {
		val, err := r.redis.Get(ctx, key).Result()
		if err == nil && val == "revoked" {
			return true, nil
		}
		if err != nil && err != redis.Nil {
			// If Redis has an error, fallback to memory
		} else {
			return false, nil
		}
	}

	// In-memory fallback
	r.mu.RLock()
	defer r.mu.RUnlock()

	exp, exists := r.memStore[tokenHash]
	if !exists {
		return false, nil
	}

	if time.Now().After(exp) {
		return false, nil
	}

	return true, nil
}
