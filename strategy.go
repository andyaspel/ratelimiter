package ratelimiter

import "time"

type Strategy interface {
	Allow(clock Clock) bool
	NextAvailable(clock Clock) time.Duration
}
