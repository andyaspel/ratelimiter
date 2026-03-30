package ratelimiter

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ClientKeyFunc derives a stable key for a request-specific rate limit bucket.
type ClientKeyFunc func(r *http.Request) string

// ClientMiddlewareConfig controls per-client HTTP rate limiting behavior.
type ClientMiddlewareConfig struct {
	Capacity         int
	RefillRate       int
	OnLimit          HTTPOnLimitFunc
	KeyFunc          ClientKeyFunc
	Clock            Clock
	TrustForwardedIP bool
	CleanupInterval  time.Duration
	EntryTTL         time.Duration
}

type clientLimiterEntry struct {
	limiter  *RateLimiter
	lastSeen time.Time
}

type clientLimiterStore struct {
	mu          sync.Mutex
	entries     map[string]*clientLimiterEntry
	cfg         ClientMiddlewareConfig
	lastCleanup time.Time
}

// NewIPRateLimitMiddleware creates plug-and-play HTTP middleware with a
// dedicated limiter per client IP address.
func NewIPRateLimitMiddleware(capacity int, refillRate int, onLimit HTTPOnLimitFunc) (func(http.Handler) http.Handler, error) {
	return NewIPRateLimitMiddlewareWithConfig(ClientMiddlewareConfig{
		Capacity:   capacity,
		RefillRate: refillRate,
		OnLimit:    onLimit,
	})
}

// NewIPRateLimitMiddlewareWithConfig creates per-client rate-limiting middleware
// with optional custom key extraction and stale bucket cleanup.
func NewIPRateLimitMiddlewareWithConfig(cfg ClientMiddlewareConfig) (func(http.Handler) http.Handler, error) {
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
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = time.Minute
	}
	if cfg.EntryTTL <= 0 {
		cfg.EntryTTL = 10 * time.Minute
	}

	store := &clientLimiterStore{
		entries:     make(map[string]*clientLimiterEntry),
		cfg:         cfg,
		lastCleanup: cfg.Clock.Now(),
	}

	return func(next http.Handler) http.Handler {
		if next == nil {
			panic("ratelimiter: next handler is nil")
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rl, err := store.limiterForRequest(r)
			if err != nil {
				http.Error(w, ErrRateLimiterUnavailable.Error(), http.StatusServiceUnavailable)
				return
			}
			if !rl.Allow() {
				setRetryAfterHeader(w, rl.NextAvailable())
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

// RealIPKeyFunc builds a client key from the request IP address. By default it
// uses RemoteAddr; when trustForwardedIP is true it will prefer X-Forwarded-For
// or X-Real-IP.
func RealIPKeyFunc(trustForwardedIP bool) ClientKeyFunc {
	return func(r *http.Request) string {
		if r == nil {
			return "unknown"
		}
		if trustForwardedIP {
			if ip := forwardedIP(r.Header.Get("X-Forwarded-For")); ip != "" {
				return ip
			}
			if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
				return ip
			}
		}

		addr := strings.TrimSpace(r.RemoteAddr)
		if addr == "" {
			return "unknown"
		}
		if host, _, err := net.SplitHostPort(addr); err == nil {
			return host
		}
		return addr
	}
}

func (s *clientLimiterStore) limiterForRequest(r *http.Request) (*RateLimiter, error) {
	key := s.cfg.KeyFunc(r)
	if key == "" {
		key = "unknown"
	}

	now := s.cfg.Clock.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if now.Sub(s.lastCleanup) >= s.cfg.CleanupInterval {
		s.cleanupLocked(now)
		s.lastCleanup = now
	}

	if entry, ok := s.entries[key]; ok {
		entry.lastSeen = now
		return entry.limiter, nil
	}

	rl, err := NewTokenBucketRateLimiterWithClock(s.cfg.Capacity, s.cfg.RefillRate, s.cfg.Clock)
	if err != nil {
		return nil, err
	}

	s.entries[key] = &clientLimiterEntry{
		limiter:  rl,
		lastSeen: now,
	}
	return rl, nil
}

func (s *clientLimiterStore) cleanupLocked(now time.Time) {
	for key, entry := range s.entries {
		if now.Sub(entry.lastSeen) > s.cfg.EntryTTL {
			delete(s.entries, key)
		}
	}
}

func forwardedIP(value string) string {
	for _, part := range strings.Split(value, ",") {
		candidate := strings.TrimSpace(part)
		if candidate != "" {
			return candidate
		}
	}
	return ""
}
