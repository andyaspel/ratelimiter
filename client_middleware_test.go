package ratelimiter

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewIPRateLimitMiddleware_ValidatesInput(t *testing.T) {
	if _, err := NewIPRateLimitMiddleware(0, 1, nil); err == nil {
		t.Fatalf("expected zero capacity to fail")
	}
	if _, err := NewIPRateLimitMiddleware(1, 0, nil); err == nil {
		t.Fatalf("expected zero refill rate to fail")
	}
}

func TestNewIPRateLimitMiddleware_SeparatesClients(t *testing.T) {
	fc := &fakeClock{now: time.Unix(0, 0)}
	mw, err := NewIPRateLimitMiddlewareWithConfig(ClientMiddlewareConfig{
		Capacity:   1,
		RefillRate: 1,
		Clock:      fc,
		KeyFunc: func(r *http.Request) string {
			return r.Header.Get("X-Client-ID")
		},
	})
	if err != nil {
		t.Fatalf("expected middleware creation to succeed: %v", err)
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

func TestNewIPRateLimitMiddleware_TrustsForwardedHeadersWhenEnabled(t *testing.T) {
	fc := &fakeClock{now: time.Unix(0, 0)}
	mw, err := NewIPRateLimitMiddlewareWithConfig(ClientMiddlewareConfig{
		Capacity:         1,
		RefillRate:       1,
		Clock:            fc,
		TrustForwardedIP: true,
	})
	if err != nil {
		t.Fatalf("expected middleware creation to succeed: %v", err)
	}

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.10:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.10, 10.0.0.10")

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, req)
	if first.Code != http.StatusOK {
		t.Fatalf("expected first forwarded request to pass, got %d", first.Code)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, req)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected forwarded client identity to be rate limited, got %d", second.Code)
	}
}

func TestRealIPKeyFunc_IgnoresForwardedHeadersByDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.10:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.10")

	if got := RealIPKeyFunc(false)(req); got != "10.0.0.10" {
		t.Fatalf("expected RemoteAddr to be used when proxy headers are untrusted, got %q", got)
	}
}

func TestNewIPRateLimitMiddleware_CleansUpIdleClients(t *testing.T) {
	fc := &fakeClock{now: time.Unix(0, 0)}
	mw, err := NewIPRateLimitMiddlewareWithConfig(ClientMiddlewareConfig{
		Capacity:        1,
		RefillRate:      1,
		Clock:           fc,
		CleanupInterval: time.Second,
		EntryTTL:        time.Second,
		KeyFunc: func(r *http.Request) string {
			return r.Header.Get("X-Client-ID")
		},
	})
	if err != nil {
		t.Fatalf("expected middleware creation to succeed: %v", err)
	}

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	reqA := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA.Header.Set("X-Client-ID", "client-a")
	handler.ServeHTTP(httptest.NewRecorder(), reqA)
	handler.ServeHTTP(httptest.NewRecorder(), reqA)

	fc.Sleep(2 * time.Second)

	reqB := httptest.NewRequest(http.MethodGet, "/", nil)
	reqB.Header.Set("X-Client-ID", "client-b")
	handler.ServeHTTP(httptest.NewRecorder(), reqB)

	retryA := httptest.NewRecorder()
	handler.ServeHTTP(retryA, reqA)
	if retryA.Code != http.StatusOK {
		t.Fatalf("expected evicted client-a to receive a fresh bucket, got %d", retryA.Code)
	}
}
