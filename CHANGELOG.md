# Changelog

All notable changes to this project will be documented in this file.

## v0.3.2

### Performance improvements

- per-client in-memory middleware now uses sharded state to reduce lock contention under high concurrency
- benchmark coverage was added for local limiter hot paths and per-client store access

## v0.3.1

### Reliability fixes

- Redis-backed middleware now returns `503 Service Unavailable` on backing-store failures instead of treating outages as normal rate limits
- SQLite file loading now reports invalid stored timestamps explicitly
- CLI help commands (`serve -h`, `save -h`, `list -h`) now exit cleanly without failing
- file-path saves detect a more accurate content type before storing to SQLite

## v0.3.0

### Added

- request logging middleware via `RequestLoggerMiddleware`
- SQLite-backed file storage for the demo executable and CLI workflows
- upload/list/download flows in `cmd/demo`

### Improved

- `.gitignore` now excludes local SQLite database files
- package docs now cover logging and SQLite usage

## v0.2.0

### Major additions

- publish-ready module metadata for `github.com/andyaspel/ratelimiter`
- plug-and-play per-client middleware helpers
- improved test coverage, race checks, and CI workflow
- optional Redis-backed distributed rate limiting support

### Notable improvements

- safer configuration validation
- better HTTP `429` behavior with `Retry-After`
- clearer package documentation and demo layout

## v0.1.0

### Initial release

- token bucket rate limiter
- `Wait()` support
- `net/http` middleware integration
