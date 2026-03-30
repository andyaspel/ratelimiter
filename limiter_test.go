package ratelimiter

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRateLimiterNilMethodsAreSafe(t *testing.T) {
	var rl *RateLimiter

	if rl.Allow() {
		t.Fatalf("expected nil limiter to deny requests")
	}
	if got := rl.NextAvailable(); got != 0 {
		t.Fatalf("expected nil limiter to report zero wait, got %v", got)
	}
}

func TestRateLimiterWaitReturnsExpectedErrors(t *testing.T) {
	var nilLimiter *RateLimiter
	if err := nilLimiter.Wait(context.Background()); !errors.Is(err, ErrNilRateLimiter) {
		t.Fatalf("expected ErrNilRateLimiter, got %v", err)
	}

	rl := &RateLimiter{clock: RealClock{}}
	if err := rl.Wait(context.Background()); !errors.Is(err, ErrNilStrategy) {
		t.Fatalf("expected ErrNilStrategy, got %v", err)
	}
}

func TestRateLimiterWaitHonorsCanceledContext(t *testing.T) {
	fc := &fakeClock{now: time.Unix(0, 0)}
	rl, err := NewTokenBucketRateLimiterWithClock(1, 1, fc)
	if err != nil {
		t.Fatalf("expected limiter creation to succeed: %v", err)
	}
	if !rl.Allow() {
		t.Fatalf("expected initial token to be available")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = rl.Wait(ctx)
	if !errors.Is(err, ErrContextCanceled) {
		t.Fatalf("expected ErrContextCanceled, got %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled to be preserved, got %v", err)
	}
}

func TestNewTokenBucketValidatedReturnsConfiguredBucket(t *testing.T) {
	tb, err := NewTokenBucketValidated(3, 2)
	if err != nil {
		t.Fatalf("expected valid token bucket config, got %v", err)
	}
	if tb.capacity != 3 || tb.tokens != 3 || tb.refillRate != 2 {
		t.Fatalf("unexpected token bucket values: %+v", tb)
	}
}
