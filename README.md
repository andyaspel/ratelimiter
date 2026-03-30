# ratelimiter

A small, concurrency-safe Go rate limiter package with plug-and-play `net/http` middleware.

## Features

- token bucket strategy
- shared, per-client, or Redis-backed HTTP middleware
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

## Logging

```go
logger := slog.Default()
handler := ratelimiter.RequestLoggerMiddleware(logger)(mux)
```

## SQLite-backed file saving demo

The bundled executable now supports both a small web UI and CLI commands for saving files into SQLite:

```bash
go run ./cmd/demo serve -db ratelimiter.db
go run ./cmd/demo save -db ratelimiter.db -file ./README.md
go run ./cmd/demo list -db ratelimiter.db
```

- `serve` starts the HTTP demo with request logging and an upload form
- `save` stores a local file in SQLite
- `list` prints saved file metadata from the database

## Redis / distributed limiting

```go
client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
middleware, err := ratelimiter.NewRedisIPRateLimitMiddleware(client, "ratelimiter:api", 20, 10, nil)
if err != nil {
    log.Fatal(err)
}

mux := http.NewServeMux()
mux.Handle("/api", middleware(http.HandlerFunc(apiHandler)))
```

Use Redis-backed middleware when your app runs across multiple instances and needs a shared global limit. If Redis becomes unavailable, the middleware now returns `503 Service Unavailable` rather than masking the outage as a normal rate limit response.

## Development checks

```bash
go test -v ./...
go test -race ./...
go vet ./...
```

## Demo

A runnable example lives in `cmd/demo/main.go`.
