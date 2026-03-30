package ratelimiter

import (
	"sync"
	"time"
)

type TokenBucket struct {
	capacity   float64
	tokens     float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

func NewTokenBucket(capacity int, refillRate int) *TokenBucket {
	tb, err := NewTokenBucketValidated(capacity, refillRate)
	if err != nil {
		panic(err)
	}

	return tb
}

func NewTokenBucketValidated(capacity int, refillRate int) (*TokenBucket, error) {
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}
	if refillRate <= 0 {
		return nil, ErrInvalidRefillRate
	}

	now := time.Now()
	return &TokenBucket{
		capacity:   float64(capacity),
		tokens:     float64(capacity),
		refillRate: float64(refillRate),
		lastRefill: now,
	}, nil
}

func (tb *TokenBucket) Allow(clock Clock) bool {
	if tb == nil || clock == nil {
		return false
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill(clock.Now())
	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

func (tb *TokenBucket) NextAvailable(clock Clock) time.Duration {
	if tb == nil || clock == nil {
		return 0
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill(clock.Now())
	if tb.tokens >= 1 {
		return 0
	}

	deficit := 1 - tb.tokens
	if tb.refillRate <= 0 {
		return 365 * 24 * time.Hour
	}

	seconds := deficit / tb.refillRate
	if seconds < 0 {
		seconds = 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func (tb *TokenBucket) refill(now time.Time) {
	elapsed := now.Sub(tb.lastRefill).Seconds()
	if elapsed <= 0 {
		return
	}

	tb.tokens = minFloat(tb.capacity, tb.tokens+elapsed*tb.refillRate)
	tb.lastRefill = now
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
