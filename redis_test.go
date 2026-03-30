package ratelimiter

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	redis "github.com/redis/go-redis/v9"
)

func TestNewRedisRateLimiter_ValidatesInput(t *testing.T) {
	if _, err := NewRedisRateLimiter(nil, "bucket", 1, 1); err == nil {
		t.Fatalf("expected nil client to fail")
	}
}

func TestNewRedisRateLimiter_AllowAndRefill(t *testing.T) {
	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("expected miniredis to start: %v", err)
	}
	defer srv.Close()

	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	defer client.Close()

	fc := &fakeClock{now: time.Unix(0, 0)}
	rl, err := NewRedisRateLimiterWithClock(client, "ratelimiter:test:bucket", 2, 1, fc)
	if err != nil {
		t.Fatalf("expected redis limiter to be created: %v", err)
	}

	if !rl.Allow() {
		t.Fatalf("expected first request to be allowed")
	}
	if !rl.Allow() {
		t.Fatalf("expected second request to be allowed")
	}
	if rl.Allow() {
		t.Fatalf("expected third request to be denied")
	}

	fc.Sleep(time.Second)
	if !rl.Allow() {
		t.Fatalf("expected request after refill to be allowed")
	}
}

func TestNewRedisIPRateLimitMiddleware_SeparatesClients(t *testing.T) {
	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("expected miniredis to start: %v", err)
	}
	defer srv.Close()

	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	defer client.Close()

	fc := &fakeClock{now: time.Unix(0, 0)}
	mw, err := NewRedisIPRateLimitMiddlewareWithConfig(RedisMiddlewareConfig{
		Client:     client,
		Prefix:     "ratelimiter:test:http",
		Capacity:   1,
		RefillRate: 1,
		Clock:      fc,
		KeyFunc: func(r *http.Request) string {
			return r.Header.Get("X-Client-ID")
		},
	})
	if err != nil {
		t.Fatalf("expected redis middleware to be created: %v", err)
	}

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	reqA := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA.Header.Set("X-Client-ID", "client-a")
	firstA := httptest.NewRecorder()
	handler.ServeHTTP(firstA, reqA)
	if firstA.Code != http.StatusOK {
		t.Fatalf("expected first client-a request to pass, got %d", firstA.Code)
	}

	secondA := httptest.NewRecorder()
	handler.ServeHTTP(secondA, reqA)
	if secondA.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second client-a request to be limited, got %d", secondA.Code)
	}

	reqB := httptest.NewRequest(http.MethodGet, "/", nil)
	reqB.Header.Set("X-Client-ID", "client-b")
	firstB := httptest.NewRecorder()
	handler.ServeHTTP(firstB, reqB)
	if firstB.Code != http.StatusOK {
		t.Fatalf("expected client-b to have its own bucket, got %d", firstB.Code)
	}
}

func TestNewRedisIPRateLimitMiddleware_ReturnsServiceUnavailableWhenRedisDown(t *testing.T) {
	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("expected miniredis to start: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr:            srv.Addr(),
		MaxRetries:      0,
		DialTimeout:     50 * time.Millisecond,
		ReadTimeout:     50 * time.Millisecond,
		WriteTimeout:    50 * time.Millisecond,
		MinRetryBackoff: -1,
		MaxRetryBackoff: -1,
	})
	defer client.Close()

	mw, err := NewRedisIPRateLimitMiddleware(client, "ratelimiter:test:http", 1, 1, nil)
	if err != nil {
		t.Fatalf("expected redis middleware to be created: %v", err)
	}

	srv.Close()

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected Redis outage to return 503, got %d", res.Code)
	}
}
