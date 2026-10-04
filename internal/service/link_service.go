package service

import (
	"context"
	"fmt"
	"regexp"
	"time"
	"url_shortener/internal/analytics"
	"url_shortener/internal/config"
	"url_shortener/internal/model/domain"
	"url_shortener/internal/model/web"
	"url_shortener/internal/repository"
	"url_shortener/pkg/errs"
	"url_shortener/pkg/geoip"
	"url_shortener/pkg/useragent"
	"url_shortener/pkg/utils"
	"url_shortener/pkg/validator"

	"github.com/bwmarrin/snowflake"
	"github.com/google/uuid"
)

var aliasRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{4,32}$`)

// LinkService defines the business logic for shortening URLs, redirection, user link management, and analytics.
type LinkService interface {
	CreateShortLink(ctx context.Context, req *web.CreateLinkRequest, userID ...*uuid.UUID) (*web.LinkResponse, error)
	GetOriginalURL(ctx context.Context, shortCode string) (string, error)
	GetAnalytics(ctx context.Context, shortCode string) (*web.AnalyticsResponse, error)
	GetUserLinks(ctx context.Context, userID uuid.UUID, page, limit int) (*web.UserLinksResponse, error)
	TrackClick(shortCode, ip, userAgent, referer, countryHeader string)
}

type linkServiceImpl struct {
	cfg         *config.Config
	repo        repository.LinkRepository
	snowNode    *snowflake.Node
	workerPool  analytics.WorkerPool
	geoResolver geoip.Resolver
	ssrfVal     validator.SSRFValidator
}

// NewLinkService creates a new LinkService instance.
func NewLinkService(
	cfg *config.Config,
	repo repository.LinkRepository,
	snowNode *snowflake.Node,
	workerPool analytics.WorkerPool,
	ssrfVal validator.SSRFValidator,
) LinkService {
	if ssrfVal == nil {
		ssrfVal = validator.NewSSRFValidator(cfg.App.BaseURL, nil)
	}

	return &linkServiceImpl{
		cfg:         cfg,
		repo:        repo,
		snowNode:    snowNode,
		workerPool:  workerPool,
		geoResolver: geoip.NewResolver("ID"),
		ssrfVal:     ssrfVal,
	}
}

func (s *linkServiceImpl) CreateShortLink(ctx context.Context, req *web.CreateLinkRequest, userID ...*uuid.UUID) (*web.LinkResponse, error) {
	// PRD Section 6.1: SSRF DNS validation, scheme whitelisting, and loop redirection check
	if s.ssrfVal != nil {
		if err := s.ssrfVal.ValidateURL(ctx, req.OriginalURL); err != nil {
			return nil, err
		}
	}

	var uID *uuid.UUID
	if len(userID) > 0 && userID[0] != nil {
		uID = userID[0]
	}

	var shortCode string

	if req.CustomAlias != "" {
		// PRD Section 3.1 & 3.4: Custom alias only available for authenticated users
		if uID == nil {
			return nil, errs.NewUnauthorizedError("custom alias is only available for authenticated users")
		}

		if !aliasRegex.MatchString(req.CustomAlias) {
			return nil, errs.NewBadRequestError("custom alias must be 4-32 characters long and contain only alphanumeric, hyphen, or underscore characters")
		}

		shortCode = req.CustomAlias
	} else {
		// Auto-generate short code using Snowflake ID + Base62
		var idInt int64
		if s.snowNode != nil {
			idInt = s.snowNode.Generate().Int64()
		} else {
			idInt = time.Now().UnixNano()
		}

		shortCode = utils.EncodeBase62(uint64(idInt))
	}

	var expiresAt *time.Time
	if req.ExpiresInHours > 0 {
		exp := time.Now().Add(time.Duration(req.ExpiresInHours) * time.Hour)
		expiresAt = &exp
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUIDv7: %w", err)
	}

	now := time.Now().UTC()
	link := &domain.Link{
		ID:          id,
		ShortCode:   shortCode,
		OriginalURL: req.OriginalURL,
		UserID:      uID,
		IsActive:    true,
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
	}

	if err := s.repo.Create(ctx, link); err != nil {
		return nil, err
	}

	// Warm cache
	ttl := 24 * time.Hour
	if expiresAt != nil {
		ttl = time.Until(*expiresAt)
	}
	_ = s.repo.SetCache(ctx, shortCode, req.OriginalURL, ttl)

	shortURL := fmt.Sprintf("%s/%s", s.cfg.App.BaseURL, shortCode)
	return &web.LinkResponse{
		ShortCode:   shortCode,
		ShortURL:    shortURL,
		OriginalURL: req.OriginalURL,
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
	}, nil
}

func (s *linkServiceImpl) GetOriginalURL(ctx context.Context, shortCode string) (string, error) {
	// 1. Try cache
	cachedURL, err := s.repo.GetCache(ctx, shortCode)
	if err == nil && cachedURL != "" {
		return cachedURL, nil
	}

	// 2. Query persistent store
	link, err := s.repo.FindByShortCode(ctx, shortCode)
	if err != nil {
		return "", err
	}

	// 3. Expiration check
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		return "", errs.NewGoneError("this link has expired")
	}

	// 4. Update cache
	ttl := 24 * time.Hour
	if link.ExpiresAt != nil {
		ttl = time.Until(*link.ExpiresAt)
	}
	_ = s.repo.SetCache(ctx, shortCode, link.OriginalURL, ttl)

	return link.OriginalURL, nil
}

func (s *linkServiceImpl) TrackClick(shortCode, ip, userAgent, referer, countryHeader string) {
	uaInfo := useragent.Parse(userAgent)
	geo := s.geoResolver.Resolve(ip, countryHeader)
	salt := s.cfg.JWT.Secret
	if salt == "" {
		salt = "default-analytics-salt"
	}
	ipHash := s.geoResolver.AnonymizeIP(ip, salt)

	event := &domain.ClickEvent{
		ShortCode:   shortCode,
		ClickedAt:   time.Now().UTC(),
		IPHash:      ipHash,
		CountryCode: geo.CountryCode,
		City:        geo.City,
		DeviceType:  uaInfo.DeviceType,
		Browser:     uaInfo.Browser,
		OS:          uaInfo.OS,
		Referer:     referer,
	}

	if s.workerPool != nil {
		s.workerPool.Enqueue(event)
	} else {
		_ = s.repo.RecordClickEvents(context.Background(), []*domain.ClickEvent{event})
	}
}

func (s *linkServiceImpl) GetAnalytics(ctx context.Context, shortCode string) (*web.AnalyticsResponse, error) {
	// Verify link exists
	_, err := s.repo.FindByShortCode(ctx, shortCode)
	if err != nil {
		return nil, err
	}

	return s.repo.GetAnalytics(ctx, shortCode)
}

func (s *linkServiceImpl) GetUserLinks(ctx context.Context, userID uuid.UUID, page, limit int) (*web.UserLinksResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	links, total, err := s.repo.FindByUserID(ctx, userID, page, limit)
	if err != nil {
		return nil, err
	}

	items := make([]web.UserLinkItem, 0, len(links))
	for _, l := range links {
		items = append(items, web.UserLinkItem{
			ID:          l.ID,
			ShortCode:   l.ShortCode,
			ShortURL:    fmt.Sprintf("%s/%s", s.cfg.App.BaseURL, l.ShortCode),
			OriginalURL: l.OriginalURL,
			IsActive:    l.IsActive,
			CreatedAt:   l.CreatedAt,
			ExpiresAt:   l.ExpiresAt,
		})
	}

	return &web.UserLinksResponse{
		Total: total,
		Page:  page,
		Limit: limit,
		Links: items,
	}, nil
}
