package ratelimiter

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeClock struct {
	now time.Time
}

func (fc *fakeClock) Now() time.Time {
	return fc.now
}

func (fc *fakeClock) Sleep(d time.Duration) {
	fc.now = fc.now.Add(d)
}

func TestTokenBucket_AllowAndRefill(t *testing.T) {
	start := time.Unix(0, 0)
	fc := &fakeClock{now: start}

	tb := &TokenBucket{
		capacity:   2,
		tokens:     2,
		refillRate: 1, // 1 token per second
		lastRefill: start,
	}

	rl := NewWithClock(tb, fc)

	if !rl.Allow() {
		t.Fatalf("expected first request to be allowed")
	}
	if !rl.Allow() {
		t.Fatalf("expected second request to be allowed")
	}
	if rl.Allow() {
		t.Fatalf("expected third request to be denied (no tokens)")
	}

	// advance 1 second -> 1 token
	fc.Sleep(time.Second)

	if !rl.Allow() {
		t.Fatalf("expected request after refill to be allowed")
	}
}

func TestTokenBucket_NextAvailableUsesElapsedTime(t *testing.T) {
	start := time.Unix(0, 0)
	fc := &fakeClock{now: start.Add(2 * time.Second)}

	tb := &TokenBucket{
		capacity:   1,
		tokens:     0,
		refillRate: 1,
		lastRefill: start,
	}

	if got := tb.NextAvailable(fc); got != 0 {
		t.Fatalf("expected no wait after enough time has elapsed, got %v", got)
	}
}

func TestNewTokenBucketRateLimiter_ValidatesInput(t *testing.T) {
	if _, err := NewTokenBucketRateLimiter(0, 1); err == nil {
		t.Fatalf("expected invalid capacity to return an error")
	}

	if _, err := NewTokenBucketRateLimiter(1, 0); err == nil {
		t.Fatalf("expected invalid refill rate to return an error")
	}
}

func TestHTTPMiddleware_SetsRetryAfterHeader(t *testing.T) {
	start := time.Unix(0, 0)
	fc := &fakeClock{now: start}

	rl, err := NewTokenBucketRateLimiterWithClock(1, 1, fc)
	if err != nil {
		t.Fatalf("expected limiter to be created: %v", err)
	}

	handler := HTTPMiddleware(rl, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, req)
	if first.Code != http.StatusOK {
		t.Fatalf("expected first request to pass, got status %d", first.Code)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, req)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request to be rate limited, got status %d", second.Code)
	}
	if got := second.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("expected Retry-After header to be set to 1, got %q", got)
	}
}

func TestHTTPMiddleware_UsesCustomOnLimitHandler(t *testing.T) {
	fc := &fakeClock{now: time.Unix(0, 0)}
	rl, err := NewTokenBucketRateLimiterWithClock(1, 1, fc)
	if err != nil {
		t.Fatalf("expected limiter to be created: %v", err)
	}

	handler := HTTPMiddleware(rl, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	limited := httptest.NewRecorder()
	handler.ServeHTTP(limited, req)
	if limited.Code != http.StatusTeapot {
		t.Fatalf("expected custom on-limit status, got %d", limited.Code)
	}
}

func TestNewTokenBucketMiddleware_ValidatesInput(t *testing.T) {
	if _, err := NewTokenBucketMiddleware(0, 1, nil); err == nil {
		t.Fatalf("expected invalid capacity to fail")
	}
}
