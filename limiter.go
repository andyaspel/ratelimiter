package ratelimiter

import (
	"context"
	"errors"
	"time"
)

type RateLimiter struct {
	strategy Strategy
	clock    Clock
}

func New(strategy Strategy) *RateLimiter {
	return NewWithClock(strategy, RealClock{})
}

func NewWithClock(strategy Strategy, clock Clock) *RateLimiter {
	if clock == nil {
		clock = RealClock{}
	}

	return &RateLimiter{
		strategy: strategy,
		clock:    clock,
	}
}

func NewTokenBucketRateLimiter(capacity int, refillRate int) (*RateLimiter, error) {
	return NewTokenBucketRateLimiterWithClock(capacity, refillRate, RealClock{})
}

func NewTokenBucketRateLimiterWithClock(capacity int, refillRate int, clock Clock) (*RateLimiter, error) {
	tb, err := NewTokenBucketValidated(capacity, refillRate)
	if err != nil {
		return nil, err
	}

	return NewWithClock(tb, clock), nil
}

func (rl *RateLimiter) Allow() bool {
	if rl == nil || rl.strategy == nil || rl.clock == nil {
		return false
	}

	return rl.strategy.Allow(rl.clock)
}

func (rl *RateLimiter) NextAvailable() time.Duration {
	if rl == nil || rl.strategy == nil || rl.clock == nil {
		return 0
	}

	return rl.strategy.NextAvailable(rl.clock)
}

func (rl *RateLimiter) Wait(ctx context.Context) error {
	if rl == nil {
		return ErrNilRateLimiter
	}
	if rl.strategy == nil {
		return ErrNilStrategy
	}
	if rl.clock == nil {
		rl.clock = RealClock{}
	}
	if ctx == nil {
		ctx = context.Background()
	}

	for {
		if rl.Allow() {
			return nil
		}

		wait := rl.NextAvailable()
		if wait <= 0 {
			continue
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(ErrContextCanceled, ctx.Err())
		case <-timer.C:
			// loop and try again
		}
	}
}
