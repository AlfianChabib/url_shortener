package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"url_shortener/internal/model/domain"
	"url_shortener/internal/model/web"
	"url_shortener/pkg/errs"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// LinkRepository defines data access methods for URL links and analytics.
type LinkRepository interface {
	Create(ctx context.Context, link *domain.Link) error
	FindByShortCode(ctx context.Context, shortCode string) (*domain.Link, error)
	FindByUserID(ctx context.Context, userID uuid.UUID, page, limit int) ([]*domain.Link, int64, error)
	GetCache(ctx context.Context, shortCode string) (string, error)
	SetCache(ctx context.Context, shortCode string, originalURL string, ttl time.Duration) error
	IncrementClick(ctx context.Context, shortCode string) error
	GetClickCount(ctx context.Context, shortCode string) (int64, error)
	RecordClickEvents(ctx context.Context, events []*domain.ClickEvent) error
	GetAnalytics(ctx context.Context, shortCode string) (*web.AnalyticsResponse, error)
}

type linkRepositoryImpl struct {
	db    *gorm.DB
	redis *redis.Client

	// In-memory fallback stores for local testing/graceful degradation
	mu        sync.RWMutex
	memLinks  map[string]*domain.Link
	memClicks map[string]int64
	memCache  map[string]string
	memEvents map[string][]*domain.ClickEvent
}

// NewLinkRepository creates a new LinkRepository instance using GORM and Redis.
func NewLinkRepository(db *gorm.DB, redisClient *redis.Client) LinkRepository {
	return &linkRepositoryImpl{
		db:        db,
		redis:     redisClient,
		memLinks:  make(map[string]*domain.Link),
		memClicks: make(map[string]int64),
		memCache:  make(map[string]string),
		memEvents: make(map[string][]*domain.ClickEvent),
	}
}

func (r *linkRepositoryImpl) Create(ctx context.Context, link *domain.Link) error {
	if r.db != nil {
		if err := r.db.WithContext(ctx).Create(link).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errs.NewConflictError("short code already exists", err)
			}
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
	if r.db != nil {
		var link domain.Link
		err := r.db.WithContext(ctx).
			Where("short_code = ? AND is_active = ?", shortCode, true).
			First(&link).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
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

func (r *linkRepositoryImpl) FindByUserID(ctx context.Context, userID uuid.UUID, page, limit int) ([]*domain.Link, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	if r.db != nil {
		var total int64
		if err := r.db.WithContext(ctx).Model(&domain.Link{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
			return nil, 0, fmt.Errorf("failed to count user links: %w", err)
		}

		offset := (page - 1) * limit
		var links []*domain.Link
		if err := r.db.WithContext(ctx).
			Where("user_id = ?", userID).
			Order("created_at DESC").
			Offset(offset).
			Limit(limit).
			Find(&links).Error; err != nil {
			return nil, 0, fmt.Errorf("failed to list user links: %w", err)
		}

		return links, total, nil
	}

	// In-memory fallback
	r.mu.RLock()
	defer r.mu.RUnlock()

	var userLinks []*domain.Link
	for _, l := range r.memLinks {
		if l.UserID != nil && *l.UserID == userID {
			userLinks = append(userLinks, l)
		}
	}

	// Sort descending by CreatedAt
	sort.Slice(userLinks, func(i, j int) bool {
		return userLinks[i].CreatedAt.After(userLinks[j].CreatedAt)
	})

	total := int64(len(userLinks))
	start := (page - 1) * limit
	if start >= len(userLinks) {
		return []*domain.Link{}, total, nil
	}
	end := start + limit
	if end > len(userLinks) {
		end = len(userLinks)
	}

	return userLinks[start:end], total, nil
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

func (r *linkRepositoryImpl) RecordClickEvents(ctx context.Context, events []*domain.ClickEvent) error {
	if len(events) == 0 {
		return nil
	}

	if r.db != nil {
		if err := r.db.WithContext(ctx).CreateInBatches(events, 100).Error; err != nil {
			return fmt.Errorf("failed to record click events: %w", err)
		}
	}

	if r.redis != nil {
		pipe := r.redis.Pipeline()
		for _, ev := range events {
			pipe.Incr(ctx, fmt.Sprintf("clicks:%s", ev.ShortCode))
		}
		_, _ = pipe.Exec(ctx)
	}

	// In-memory fallback
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ev := range events {
		r.memEvents[ev.ShortCode] = append(r.memEvents[ev.ShortCode], ev)
		r.memClicks[ev.ShortCode]++
	}

	return nil
}

func (r *linkRepositoryImpl) GetAnalytics(ctx context.Context, shortCode string) (*web.AnalyticsResponse, error) {
	if r.db != nil {
		var totalClicks int64
		if err := r.db.WithContext(ctx).Model(&domain.ClickEvent{}).Where("short_code = ?", shortCode).Count(&totalClicks).Error; err != nil {
			return nil, fmt.Errorf("failed to count total clicks: %w", err)
		}

		var uniqueClicks int64
		if err := r.db.WithContext(ctx).Model(&domain.ClickEvent{}).Where("short_code = ?", shortCode).Distinct("ip_hash").Count(&uniqueClicks).Error; err != nil {
			return nil, fmt.Errorf("failed to count unique clicks: %w", err)
		}

		var topCountries []web.CountryClick
		if err := r.db.WithContext(ctx).Model(&domain.ClickEvent{}).
			Select("country_code as code, count(*) as clicks").
			Where("short_code = ?", shortCode).
			Group("country_code").
			Order("clicks DESC").
			Limit(10).
			Scan(&topCountries).Error; err != nil {
			return nil, fmt.Errorf("failed to query top countries: %w", err)
		}
		if topCountries == nil {
			topCountries = []web.CountryClick{}
		}

		type deviceStat struct {
			DeviceType string
			Count      int64
		}
		var devStats []deviceStat
		if err := r.db.WithContext(ctx).Model(&domain.ClickEvent{}).
			Select("device_type, count(*) as count").
			Where("short_code = ?", shortCode).
			Group("device_type").
			Scan(&devStats).Error; err != nil {
			return nil, fmt.Errorf("failed to query device stats: %w", err)
		}

		var mobileCount, desktopCount, botCount int64
		for _, ds := range devStats {
			switch strings.ToLower(ds.DeviceType) {
			case "mobile":
				mobileCount += ds.Count
			case "desktop":
				desktopCount += ds.Count
			case "bot":
				botCount += ds.Count
			default:
				desktopCount += ds.Count
			}
		}

		var devResp web.DevicesStats
		if totalClicks > 0 {
			devResp.Mobile = math.Round((float64(mobileCount)/float64(totalClicks)*100)*10) / 10
			devResp.Desktop = math.Round((float64(desktopCount)/float64(totalClicks)*100)*10) / 10
			devResp.Bot = math.Round((float64(botCount)/float64(totalClicks)*100)*10) / 10
		}

		type tsRow struct {
			Bucket time.Time
			Count  int64
		}
		var tsRows []tsRow
		if err := r.db.WithContext(ctx).Model(&domain.ClickEvent{}).
			Select("date_trunc('hour', clicked_at) as bucket, count(*) as count").
			Where("short_code = ?", shortCode).
			Group("bucket").
			Order("bucket ASC").
			Scan(&tsRows).Error; err != nil {
			return nil, fmt.Errorf("failed to query timeseries: %w", err)
		}

		timeSeries := make([]web.TimeSeriesData, 0, len(tsRows))
		for _, row := range tsRows {
			timeSeries = append(timeSeries, web.TimeSeriesData{
				Timestamp: row.Bucket,
				Count:     row.Count,
			})
		}

		return &web.AnalyticsResponse{
			ShortCode:    shortCode,
			TotalClicks:  totalClicks,
			UniqueClicks: uniqueClicks,
			TopCountries: topCountries,
			Devices:      devResp,
			TimeSeries:   timeSeries,
		}, nil
	}

	// In-memory fallback
	r.mu.RLock()
	defer r.mu.RUnlock()

	events := r.memEvents[shortCode]
	totalClicks := int64(len(events))

	ipSet := make(map[string]struct{})
	countryMap := make(map[string]int64)
	deviceMap := make(map[string]int64)
	tsMap := make(map[time.Time]int64)

	for _, ev := range events {
		ipSet[ev.IPHash] = struct{}{}
		code := ev.CountryCode
		if code == "" {
			code = "ID"
		}
		countryMap[code]++
		deviceMap[strings.ToLower(ev.DeviceType)]++
		bucket := ev.ClickedAt.Truncate(time.Hour)
		tsMap[bucket]++
	}

	uniqueClicks := int64(len(ipSet))

	var topCountries []web.CountryClick
	for code, cnt := range countryMap {
		topCountries = append(topCountries, web.CountryClick{
			Code:   code,
			Clicks: cnt,
		})
	}
	sort.Slice(topCountries, func(i, j int) bool {
		return topCountries[i].Clicks > topCountries[j].Clicks
	})
	if len(topCountries) > 10 {
		topCountries = topCountries[:10]
	}
	if topCountries == nil {
		topCountries = []web.CountryClick{}
	}

	var devResp web.DevicesStats
	if totalClicks > 0 {
		mobile := float64(deviceMap["mobile"])
		desktop := float64(deviceMap["desktop"])
		bot := float64(deviceMap["bot"])

		devResp.Mobile = math.Round((mobile/float64(totalClicks)*100)*10) / 10
		devResp.Desktop = math.Round((desktop/float64(totalClicks)*100)*10) / 10
		devResp.Bot = math.Round((bot/float64(totalClicks)*100)*10) / 10
	}

	var timeSeries []web.TimeSeriesData
	for ts, cnt := range tsMap {
		timeSeries = append(timeSeries, web.TimeSeriesData{
			Timestamp: ts,
			Count:     cnt,
		})
	}
	sort.Slice(timeSeries, func(i, j int) bool {
		return timeSeries[i].Timestamp.Before(timeSeries[j].Timestamp)
	})
	if timeSeries == nil {
		timeSeries = []web.TimeSeriesData{}
	}

	return &web.AnalyticsResponse{
		ShortCode:    shortCode,
		TotalClicks:  totalClicks,
		UniqueClicks: uniqueClicks,
		TopCountries: topCountries,
		Devices:      devResp,
		TimeSeries:   timeSeries,
	}, nil
}
