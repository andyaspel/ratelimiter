package ratelimiter

import "errors"

var (
	ErrContextCanceled   = errors.New("ratelimiter: context canceled")
	ErrNilRateLimiter    = errors.New("ratelimiter: nil rate limiter")
	ErrNilStrategy       = errors.New("ratelimiter: nil strategy")
	ErrInvalidCapacity   = errors.New("ratelimiter: capacity must be greater than zero")
	ErrInvalidRefillRate = errors.New("ratelimiter: refill rate must be greater than zero")
)