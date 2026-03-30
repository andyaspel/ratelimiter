# Changelog

All notable changes to this project will be documented in this file.

## v0.2.0

### Added

- publish-ready module metadata for `github.com/andyaspel/ratelimiter`
- plug-and-play per-client middleware helpers
- improved test coverage, race checks, and CI workflow
- optional Redis-backed distributed rate limiting support

### Improved

- safer configuration validation
- better HTTP `429` behavior with `Retry-After`
- clearer package documentation and demo layout

## v0.1.0

### Initial release

- token bucket rate limiter
- `Wait()` support
- `net/http` middleware integration
