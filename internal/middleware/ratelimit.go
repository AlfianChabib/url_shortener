package middleware

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
	"url_shortener/pkg/errs"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RateLimitResult contains metadata about the rate limit decision.
type RateLimitResult struct {
	Allowed   bool
	Limit     int64
	Remaining int64
	Reset     time.Duration
}

// RateLimiter defines the interface for checking and enforcing rate limits.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int64, window time.Duration) (*RateLimitResult, error)
}

// SlidingWindowLimiter implements RateLimiter using Redis or in-memory fallback.
type SlidingWindowLimiter struct {
	redis *redis.Client

	// In-memory fallback
	mu       sync.Mutex
	memStore map[string][]time.Time
}

// NewRateLimiter creates a new RateLimiter instance.
func NewRateLimiter(redisClient *redis.Client) RateLimiter {
	return &SlidingWindowLimiter{
		redis:    redisClient,
		memStore: make(map[string][]time.Time),
	}
}

var redisSlidingWindowScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local clearBefore = now - window

redis.call('ZREMRANGEBYSCORE', key, 0, clearBefore)
local currentCount = redis.call('ZCARD', key)

if currentCount < limit then
    redis.call('ZADD', key, now, now)
    redis.call('PEXPIRE', key, window)
    return {1, limit - currentCount - 1, math.ceil(window / 1000)}
else
    local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
    local resetMs = window
    if oldest and #oldest >= 2 then
        local oldestTime = tonumber(oldest[2])
        resetMs = math.max(0, (oldestTime + window) - now)
    end
    return {0, 0, math.ceil(resetMs / 1000)}
end
`)

// Allow evaluates whether a request identified by key is permitted under limit within window.
func (l *SlidingWindowLimiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (*RateLimitResult, error) {
	now := time.Now()

	// Try Redis if available
	if l.redis != nil {
		nowMs := now.UnixNano() / int64(time.Millisecond)
		windowMs := int64(window / time.Millisecond)

		res, err := redisSlidingWindowScript.Run(ctx, l.redis, []string{key}, nowMs, windowMs, limit).Slice()
		if err == nil && len(res) == 3 {
			allowed := res[0].(int64) == 1
			remaining := res[1].(int64)
			resetSec := res[2].(int64)

			return &RateLimitResult{
				Allowed:   allowed,
				Limit:     limit,
				Remaining: remaining,
				Reset:     time.Duration(resetSec) * time.Second,
			}, nil
		}
	}

	// In-memory sliding window fallback
	l.mu.Lock()
	defer l.mu.Unlock()

	threshold := now.Add(-window)
	timestamps := l.memStore[key]

	// Prune timestamps older than window
	var valid []time.Time
	for _, ts := range timestamps {
		if ts.After(threshold) {
			valid = append(valid, ts)
		}
	}

	if int64(len(valid)) < limit {
		valid = append(valid, now)
		l.memStore[key] = valid
		remaining := limit - int64(len(valid))
		return &RateLimitResult{
			Allowed:   true,
			Limit:     limit,
			Remaining: remaining,
			Reset:     window,
		}, nil
	}

	// Limit reached
	l.memStore[key] = valid
	var resetDur time.Duration = window
	if len(valid) > 0 {
		resetDur = valid[0].Add(window).Sub(now)
		if resetDur < 0 {
			resetDur = time.Second
		}
	}

	return &RateLimitResult{
		Allowed:   false,
		Limit:     limit,
		Remaining: 0,
		Reset:     resetDur,
	}, nil
}

// GetClientIP extracts the client IP address from context.
func GetClientIP(c fiber.Ctx) string {
	if xff := c.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}
	ip := c.IP()
	if ip == "" {
		return "127.0.0.1"
	}
	return ip
}

// GetIPv4Subnet24 returns the /24 subnet for IPv4 addresses, or the IP itself for IPv6.
func GetIPv4Subnet24(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ipStr
	}
	if ip4 := ip.To4(); ip4 != nil {
		mask := net.CIDRMask(24, 32)
		network := ip4.Mask(mask)
		return fmt.Sprintf("%s/24", network.String())
	}
	return ipStr
}

func applyRateLimitHeaders(c fiber.Ctx, res *RateLimitResult) {
	c.Set("X-RateLimit-Limit", strconv.FormatInt(res.Limit, 10))
	c.Set("X-RateLimit-Remaining", strconv.FormatInt(res.Remaining, 10))
	resetSec := int64(res.Reset.Seconds())
	if resetSec < 1 {
		resetSec = 1
	}
	c.Set("X-RateLimit-Reset", strconv.FormatInt(resetSec, 10))
	if !res.Allowed {
		c.Set("Retry-After", strconv.FormatInt(resetSec, 10))
	}
}

// NewLinkCreationRateLimiter enforces PRD 6.2 rate limits for POST /api/v1/links:
// - Anonymous Mode: 10 requests / minute per IP
// - Authenticated User Mode: 1000 requests / minute per User ID
func NewLinkCreationRateLimiter(limiter RateLimiter) fiber.Handler {
	return func(c fiber.Ctx) error {
		var key string
		var limit int64 = 10
		window := 1 * time.Minute

		if val := c.Locals("user_id"); val != nil {
			if uid, ok := val.(uuid.UUID); ok && uid != uuid.Nil {
				key = "ratelimit:links:user:" + uid.String()
				limit = 1000
			}
		}

		if key == "" {
			clientIP := GetClientIP(c)
			key = "ratelimit:links:anon:" + clientIP
			limit = 10
		}

		res, err := limiter.Allow(c.Context(), key, limit, window)
		if err != nil {
			return c.Next()
		}

		applyRateLimitHeaders(c, res)

		if !res.Allowed {
			return errs.NewTooManyRequestsError("rate limit exceeded for link creation, please try again later")
		}

		return c.Next()
	}
}

// NewAuthLoginRateLimiter enforces PRD 6.2 rate limit for POST /api/v1/auth/login:
// - 5 attempts per 5 minutes per IP
func NewAuthLoginRateLimiter(limiter RateLimiter) fiber.Handler {
	return func(c fiber.Ctx) error {
		clientIP := GetClientIP(c)
		key := "ratelimit:auth:login:" + clientIP
		limit := int64(5)
		window := 5 * time.Minute

		res, err := limiter.Allow(c.Context(), key, limit, window)
		if err != nil {
			return c.Next()
		}

		applyRateLimitHeaders(c, res)

		if !res.Allowed {
			return errs.NewTooManyRequestsError("too many login attempts, please try again after 5 minutes")
		}

		return c.Next()
	}
}

// NewAuthRegisterRateLimiter enforces PRD 6.2 rate limit for POST /api/v1/auth/register:
// - 3 registrations per hour per IP
func NewAuthRegisterRateLimiter(limiter RateLimiter) fiber.Handler {
	return func(c fiber.Ctx) error {
		clientIP := GetClientIP(c)
		key := "ratelimit:auth:register:" + clientIP
		limit := int64(3)
		window := 1 * time.Hour

		res, err := limiter.Allow(c.Context(), key, limit, window)
		if err != nil {
			return c.Next()
		}

		applyRateLimitHeaders(c, res)

		if !res.Allowed {
			return errs.NewTooManyRequestsError("registration rate limit exceeded, maximum 3 registrations per hour")
		}

		return c.Next()
	}
}

// NewRedirectRateLimiter enforces PRD 6.2 rate limit for GET /{short_code}:
// - 100 requests / second per /24 IPv4 subnet
func NewRedirectRateLimiter(limiter RateLimiter) fiber.Handler {
	return func(c fiber.Ctx) error {
		clientIP := GetClientIP(c)
		subnet := GetIPv4Subnet24(clientIP)
		key := "ratelimit:redirect:" + subnet
		limit := int64(100)
		window := 1 * time.Second

		res, err := limiter.Allow(c.Context(), key, limit, window)
		if err != nil {
			return c.Next()
		}

		applyRateLimitHeaders(c, res)

		if !res.Allowed {
			return errs.NewTooManyRequestsError("redirection rate limit exceeded, please slow down")
		}

		return c.Next()
	}
}
