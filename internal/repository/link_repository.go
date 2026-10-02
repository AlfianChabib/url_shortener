package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
	"url_shortener/internal/model/domain"
	"url_shortener/pkg/errs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// LinkRepository defines data access methods for URL links and analytics.
type LinkRepository interface {
	Create(ctx context.Context, link *domain.Link) error
	FindByShortCode(ctx context.Context, shortCode string) (*domain.Link, error)
	GetCache(ctx context.Context, shortCode string) (string, error)
	SetCache(ctx context.Context, shortCode string, originalURL string, ttl time.Duration) error
	IncrementClick(ctx context.Context, shortCode string) error
	GetClickCount(ctx context.Context, shortCode string) (int64, error)
}

type linkRepositoryImpl struct {
	pool  *pgxpool.Pool
	redis *redis.Client

	// In-memory fallback stores for local testing/graceful degradation
	mu        sync.RWMutex
	memLinks  map[string]*domain.Link
	memClicks map[string]int64
	memCache  map[string]string
}

// NewLinkRepository creates a new LinkRepository instance.
func NewLinkRepository(pool *pgxpool.Pool, redisClient *redis.Client) LinkRepository {
	return &linkRepositoryImpl{
		pool:      pool,
		redis:     redisClient,
		memLinks:  make(map[string]*domain.Link),
		memClicks: make(map[string]int64),
		memCache:  make(map[string]string),
	}
}

func (r *linkRepositoryImpl) Create(ctx context.Context, link *domain.Link) error {
	if r.pool != nil {
		query := `
			INSERT INTO links (id, short_code, original_url, user_id, is_active, created_at, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`
		_, err := r.pool.Exec(ctx, query,
			link.ID,
			link.ShortCode,
			link.OriginalURL,
			link.UserID,
			link.IsActive,
			link.CreatedAt,
			link.ExpiresAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert link: %w", err)
		}
		return nil
	}

	// In-memory fallback
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.memLinks[link.ShortCode]; exists {
		return errs.NewConflictError("short code already exists")
	}

	r.memLinks[link.ShortCode] = link
	return nil
}

func (r *linkRepositoryImpl) FindByShortCode(ctx context.Context, shortCode string) (*domain.Link, error) {
	if r.pool != nil {
		query := `
			SELECT id, short_code, original_url, user_id, is_active, created_at, expires_at
			FROM links
			WHERE short_code = $1 AND is_active = true
		`
		var link domain.Link
		err := r.pool.QueryRow(ctx, query, shortCode).Scan(
			&link.ID,
			&link.ShortCode,
			&link.OriginalURL,
			&link.UserID,
			&link.IsActive,
			&link.CreatedAt,
			&link.ExpiresAt,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errs.NewNotFoundError("link not found")
			}
			return nil, fmt.Errorf("failed to query link: %w", err)
		}
		return &link, nil
	}

	// In-memory fallback
	r.mu.RLock()
	defer r.mu.RUnlock()

	link, exists := r.memLinks[shortCode]
	if !exists || !link.IsActive {
		return nil, errs.NewNotFoundError("link not found")
	}

	return link, nil
}

func (r *linkRepositoryImpl) GetCache(ctx context.Context, shortCode string) (string, error) {
	if r.redis != nil {
		key := fmt.Sprintf("link:%s", shortCode)
		val, err := r.redis.Get(ctx, key).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return "", nil
			}
			return "", err
		}
		return val, nil
	}

	// In-memory fallback
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.memCache[shortCode], nil
}

func (r *linkRepositoryImpl) SetCache(ctx context.Context, shortCode string, originalURL string, ttl time.Duration) error {
	if r.redis != nil {
		key := fmt.Sprintf("link:%s", shortCode)
		return r.redis.Set(ctx, key, originalURL, ttl).Err()
	}

	// In-memory fallback
	r.mu.Lock()
	defer r.mu.Unlock()
	r.memCache[shortCode] = originalURL
	return nil
}

func (r *linkRepositoryImpl) IncrementClick(ctx context.Context, shortCode string) error {
	if r.redis != nil {
		key := fmt.Sprintf("clicks:%s", shortCode)
		return r.redis.Incr(ctx, key).Err()
	}

	// In-memory fallback
	r.mu.Lock()
	defer r.mu.Unlock()
	r.memClicks[shortCode]++
	return nil
}

func (r *linkRepositoryImpl) GetClickCount(ctx context.Context, shortCode string) (int64, error) {
	if r.redis != nil {
		key := fmt.Sprintf("clicks:%s", shortCode)
		count, err := r.redis.Get(ctx, key).Int64()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return 0, nil
			}
			return 0, err
		}
		return count, nil
	}

	// In-memory fallback
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.memClicks[shortCode], nil
}

