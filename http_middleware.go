package ratelimiter

import (
	"math"
	"net/http"
	"strconv"
	"time"
)

type HTTPOnLimitFunc func(w http.ResponseWriter, r *http.Request)

func HTTPMiddleware(rl *RateLimiter, onLimit HTTPOnLimitFunc) func(http.Handler) http.Handler {
	if rl == nil {
		panic("ratelimiter: RateLimiter is nil")
	}

	return func(next http.Handler) http.Handler {
		if next == nil {
			panic("ratelimiter: next handler is nil")
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.Allow() {
				setRetryAfterHeader(w, rl.NextAvailable())
				if onLimit != nil {
					onLimit(w, r)
				} else {
					http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func NewTokenBucketMiddleware(capacity int, refillRate int, onLimit HTTPOnLimitFunc) (func(http.Handler) http.Handler, error) {
	rl, err := NewTokenBucketRateLimiter(capacity, refillRate)
	if err != nil {
		return nil, err
	}

	return HTTPMiddleware(rl, onLimit), nil
}

func setRetryAfterHeader(w http.ResponseWriter, d time.Duration) {
	if d <= 0 {
		return
	}

	seconds := int(math.Ceil(d.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
}
