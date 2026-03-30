package ratelimiter

import (
	"net/http"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// RedisMiddlewareConfig configures distributed per-client rate limiting backed
// by Redis.
type RedisMiddlewareConfig struct {
	Client           redis.UniversalClient
	Prefix           string
	Capacity         int
	RefillRate       int
	OnLimit          HTTPOnLimitFunc
	KeyFunc          ClientKeyFunc
	Clock            Clock
	TrustForwardedIP bool
	EntryTTL         time.Duration
}

// NewRedisIPRateLimitMiddleware creates plug-and-play per-client middleware
// using Redis as the shared backing store.
func NewRedisIPRateLimitMiddleware(client redis.UniversalClient, prefix string, capacity int, refillRate int, onLimit HTTPOnLimitFunc) (func(http.Handler) http.Handler, error) {
	return NewRedisIPRateLimitMiddlewareWithConfig(RedisMiddlewareConfig{
		Client:     client,
		Prefix:     prefix,
		Capacity:   capacity,
		RefillRate: refillRate,
		OnLimit:    onLimit,
	})
}

// NewRedisIPRateLimitMiddlewareWithConfig creates distributed per-client HTTP
// middleware with shared Redis state, suitable for multi-instance deployments.
func NewRedisIPRateLimitMiddlewareWithConfig(cfg RedisMiddlewareConfig) (func(http.Handler) http.Handler, error) {
	if cfg.Client == nil {
		return nil, ErrNilRedisClient
	}
	if cfg.Capacity <= 0 {
		return nil, ErrInvalidCapacity
	}
	if cfg.RefillRate <= 0 {
		return nil, ErrInvalidRefillRate
	}
	if cfg.Clock == nil {
		cfg.Clock = RealClock{}
	}
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = RealIPKeyFunc(cfg.TrustForwardedIP)
	}
	if strings.TrimSpace(cfg.Prefix) == "" {
		cfg.Prefix = "ratelimiter"
	}
	if cfg.EntryTTL <= 0 {
		cfg.EntryTTL = defaultRedisKeyTTL(cfg.Capacity, cfg.RefillRate)
	}

	return func(next http.Handler) http.Handler {
		if next == nil {
			panic("ratelimiter: next handler is nil")
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := cfg.KeyFunc(r)
			if key == "" {
				key = "unknown"
			}

			rl, err := NewRedisRateLimiterWithClockAndTTL(
				cfg.Client,
				cfg.Prefix+":"+key,
				cfg.Capacity,
				cfg.RefillRate,
				cfg.Clock,
				cfg.EntryTTL,
			)
			if err != nil || !rl.Allow() {
				if rl != nil {
					setRetryAfterHeader(w, rl.NextAvailable())
				}
				if cfg.OnLimit != nil {
					cfg.OnLimit(w, r)
				} else {
					http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}, nil
}
