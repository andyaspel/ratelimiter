package ratelimiter

import (
	"net/http"
	"strconv"
	"testing"
	"time"
)

func BenchmarkTokenBucketAllowParallel(b *testing.B) {
	tb := &TokenBucket{
		capacity:   1 << 30,
		tokens:     1 << 30,
		refillRate: 1,
		lastRefill: time.Now(),
	}
	rl := New(tb)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = rl.Allow()
		}
	})
}

func BenchmarkClientLimiterStoreParallelSingleKey(b *testing.B) {
	fc := &fakeClock{now: time.Unix(0, 0)}
	store := newClientLimiterStore(ClientMiddlewareConfig{
		Capacity:        1 << 30,
		RefillRate:      1,
		Clock:           fc,
		CleanupInterval: time.Hour,
		EntryTTL:        time.Hour,
		Shards:          32,
		KeyFunc: func(r *http.Request) string {
			return r.Header.Get("X-Client-ID")
		},
	})

	req := &http.Request{Header: http.Header{"X-Client-ID": []string{"hot-client"}}}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := store.limiterForRequest(req)
			if err != nil {
				b.Fatalf("unexpected limiter creation error: %v", err)
			}
		}
	})
}

func BenchmarkClientLimiterStoreParallelManyKeys(b *testing.B) {
	fc := &fakeClock{now: time.Unix(0, 0)}
	store := newClientLimiterStore(ClientMiddlewareConfig{
		Capacity:        1 << 30,
		RefillRate:      1,
		Clock:           fc,
		CleanupInterval: time.Hour,
		EntryTTL:        time.Hour,
		Shards:          32,
		KeyFunc: func(r *http.Request) string {
			return r.Header.Get("X-Client-ID")
		},
	})

	reqs := make([]*http.Request, 1024)
	for i := range reqs {
		reqs[i] = &http.Request{Header: http.Header{"X-Client-ID": []string{"client-" + strconv.Itoa(i)}}}
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, err := store.limiterForRequest(reqs[i%len(reqs)])
			if err != nil {
				b.Fatalf("unexpected limiter creation error: %v", err)
			}
			i++
		}
	})
}
