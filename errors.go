package ratelimiter

import "errors"

var (
	ErrContextCanceled        = errors.New("ratelimiter: context canceled")
	ErrNilRateLimiter         = errors.New("ratelimiter: nil rate limiter")
	ErrNilStrategy            = errors.New("ratelimiter: nil strategy")
	ErrNilSQLiteStore         = errors.New("ratelimiter: nil SQLite store")
	ErrNilRedisClient         = errors.New("ratelimiter: nil Redis client")
	ErrEmptyFileName          = errors.New("ratelimiter: file name must not be empty")
	ErrEmptyRedisKey          = errors.New("ratelimiter: Redis key must not be empty")
	ErrInvalidCapacity        = errors.New("ratelimiter: capacity must be greater than zero")
	ErrInvalidRefillRate      = errors.New("ratelimiter: refill rate must be greater than zero")
	ErrRateLimiterUnavailable = errors.New("ratelimiter: rate limiter unavailable")
	ErrInvalidStoredTimestamp = errors.New("ratelimiter: invalid stored timestamp")
)
