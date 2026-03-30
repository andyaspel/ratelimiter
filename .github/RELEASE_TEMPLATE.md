# Release template

## Release title

`v0.3.0` — logged SQLite/Redis-ready Go rate limiter

## Summary

Small, concurrency-safe Go rate limiting package with token bucket logic, HTTP middleware, Redis-backed distributed limiting, and a demo executable that logs requests and saves files to SQLite.

## Highlights

- token bucket limiter with `Wait()` support
- request logging middleware via `RequestLoggerMiddleware`
- SQLite-backed demo UI and CLI for saving files locally
- per-client IP middleware with safe proxy handling
- Redis-backed distributed middleware for multi-instance deployments
- CI, tests, race checks, and documentation

## Suggested repository description

Concurrency-safe Go rate limiter with token bucket, request logging, SQLite demo storage, per-client IP middleware, and optional Redis support.

## Suggested topics

`go`, `golang`, `rate-limiter`, `middleware`, `token-bucket`, `sqlite`, `redis`, `http`
