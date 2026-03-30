/*
Package ratelimiter provides a concurrency-safe token bucket rate limiter
with plug-and-play net/http middleware.

Install:

	go get github.com/andyaspel/ratelimiter

Quick start:

	rl, err := ratelimiter.NewTokenBucketRateLimiter(10, 5)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", ratelimiter.HTTPMiddleware(rl, nil)(http.HandlerFunc(handler)))

Per-client IP limiting:

	middleware, err := ratelimiter.NewIPRateLimitMiddleware(10, 5, nil)
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", middleware(http.HandlerFunc(handler)))

For distributed deployments, use the Redis-backed middleware helpers.
*/
package ratelimiter
