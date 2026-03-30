# Release template

## Release title

`v0.2.0` — standalone Go rate limiter with Redis support

## Summary

Small, concurrency-safe Go rate limiting package with token bucket logic, shared and per-client HTTP middleware, and optional Redis-backed distributed limiting.

## Highlights

- token bucket limiter with `Wait()` support
- `Retry-After` support for HTTP `429` responses
- per-client IP middleware with safe proxy handling
- Redis-backed distributed middleware for multi-instance deployments
- CI, tests, race checks, and documentation

## Suggested repository description

Concurrency-safe Go rate limiter with token bucket, HTTP middleware, per-client IP limiting, and optional Redis support.

## Suggested topics

`go`, `golang`, `rate-limiter`, `middleware`, `token-bucket`, `redis`, `http`
