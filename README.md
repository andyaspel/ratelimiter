# ratelimiter

A small, concurrency-safe Go rate limiter package with plug-and-play `net/http` middleware.

## Features

- token bucket strategy
- shared or per-client HTTP middleware
- `Retry-After` support on `429 Too Many Requests`
- input validation for safer configuration
- race-tested, unit-tested behavior

## Install

```bash
go get github.com/andyaspel/ratelimiter
```

## Quick start

```go
package main

import (
    "log"
    "net/http"

    ratelimiter "github.com/andyaspel/ratelimiter"
)

func main() {
    rl, err := ratelimiter.NewTokenBucketRateLimiter(10, 5)
    if err != nil {
        log.Fatal(err)
    }

    mux := http.NewServeMux()
    mux.Handle("/", ratelimiter.HTTPMiddleware(rl, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("ok"))
    })))

    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

## Per-client / plug-and-play middleware

```go
middleware, err := ratelimiter.NewIPRateLimitMiddleware(20, 10, nil)
if err != nil {
    log.Fatal(err)
}

mux := http.NewServeMux()
mux.Handle("/api", middleware(http.HandlerFunc(apiHandler)))
```

If you run behind a trusted reverse proxy, use `NewIPRateLimitMiddlewareWithConfig` and set `TrustForwardedIP: true`.

## Development checks

```bash
go test -v ./...
go test -race ./...
go vet ./...
```

## Demo

A runnable example lives in `cmd/demo/main.go`.
