package ratelimiter

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
)

var redisTokenBucketScript = redis.NewScript(`
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local consume = tonumber(ARGV[4])
local ttl = tonumber(ARGV[5])

local data = redis.call("HMGET", KEYS[1], "tokens", "last")
local tokens = tonumber(data[1])
local last = tonumber(data[2])

if tokens == nil then tokens = capacity end
if last == nil then last = now end

local delta = now - last
if delta < 0 then delta = 0 end

tokens = math.min(capacity, tokens + delta * refill_rate)

local allowed = 0
if consume == 1 then
    if tokens >= 1 then
        tokens = tokens - 1
        allowed = 1
    end
else
    if tokens >= 1 then
        allowed = 1
    end
end

local wait = 0
if tokens < 1 then
    wait = (1 - tokens) / refill_rate
end

redis.call("HSET", KEYS[1], "tokens", tokens, "last", now)
redis.call("EXPIRE", KEYS[1], ttl)
return {allowed, wait}
`)

// RedisTokenBucket stores token bucket state in Redis so multiple app instances
// can share a single rate limit.
type RedisTokenBucket struct {
	client     redis.UniversalClient
	key        string
	capacity   int
	refillRate int
	ttl        time.Duration
}

func NewRedisTokenBucket(client redis.UniversalClient, key string, capacity int, refillRate int) (*RedisTokenBucket, error) {
	return NewRedisTokenBucketWithTTL(client, key, capacity, refillRate, 0)
}

func NewRedisTokenBucketWithTTL(client redis.UniversalClient, key string, capacity int, refillRate int, ttl time.Duration) (*RedisTokenBucket, error) {
	if client == nil {
		return nil, ErrNilRedisClient
	}
	if strings.TrimSpace(key) == "" {
		return nil, ErrEmptyRedisKey
	}
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}
	if refillRate <= 0 {
		return nil, ErrInvalidRefillRate
	}
	if ttl <= 0 {
		ttl = defaultRedisKeyTTL(capacity, refillRate)
	}

	return &RedisTokenBucket{
		client:     client,
		key:        strings.TrimSpace(key),
		capacity:   capacity,
		refillRate: refillRate,
		ttl:        ttl,
	}, nil
}

func NewRedisRateLimiter(client redis.UniversalClient, key string, capacity int, refillRate int) (*RateLimiter, error) {
	return NewRedisRateLimiterWithClockAndTTL(client, key, capacity, refillRate, RealClock{}, 0)
}

func NewRedisRateLimiterWithClock(client redis.UniversalClient, key string, capacity int, refillRate int, clock Clock) (*RateLimiter, error) {
	return NewRedisRateLimiterWithClockAndTTL(client, key, capacity, refillRate, clock, 0)
}

func NewRedisRateLimiterWithClockAndTTL(client redis.UniversalClient, key string, capacity int, refillRate int, clock Clock, ttl time.Duration) (*RateLimiter, error) {
	strategy, err := NewRedisTokenBucketWithTTL(client, key, capacity, refillRate, ttl)
	if err != nil {
		return nil, err
	}
	return NewWithClock(strategy, clock), nil
}

func (tb *RedisTokenBucket) Allow(clock Clock) bool {
	allowed, _, err := tb.run(clock, true)
	return err == nil && allowed
}

func (tb *RedisTokenBucket) NextAvailable(clock Clock) time.Duration {
	_, wait, err := tb.run(clock, false)
	if err != nil {
		return 0
	}
	return wait
}

func (tb *RedisTokenBucket) run(clock Clock, consume bool) (bool, time.Duration, error) {
	if tb == nil || clock == nil {
		return false, 0, nil
	}

	consumeFlag := 0
	if consume {
		consumeFlag = 1
	}

	cmd := redisTokenBucketScript.Run(
		context.Background(),
		tb.client,
		[]string{tb.key},
		tb.capacity,
		tb.refillRate,
		float64(clock.Now().UnixNano())/float64(time.Second),
		consumeFlag,
		ttlSeconds(tb.ttl),
	)

	values, err := cmd.Slice()
	if err != nil {
		return false, 0, err
	}
	if len(values) != 2 {
		return false, 0, fmt.Errorf("ratelimiter: unexpected Redis script response length %d", len(values))
	}

	allowedValue, err := redisFloat(values[0])
	if err != nil {
		return false, 0, err
	}
	waitValue, err := redisFloat(values[1])
	if err != nil {
		return false, 0, err
	}

	wait := time.Duration(waitValue * float64(time.Second))
	if wait < 0 {
		wait = 0
	}

	return allowedValue >= 1, wait, nil
}

func defaultRedisKeyTTL(capacity int, refillRate int) time.Duration {
	secondsToFull := int(math.Ceil(float64(capacity) / float64(refillRate)))
	if secondsToFull < 60 {
		secondsToFull = 60
	}
	return time.Duration(secondsToFull*2) * time.Second
}

func ttlSeconds(ttl time.Duration) int {
	seconds := int(math.Ceil(ttl.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return seconds
}

func redisFloat(value any) (float64, error) {
	switch v := value.(type) {
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	case []byte:
		return strconv.ParseFloat(string(v), 64)
	default:
		return 0, fmt.Errorf("ratelimiter: unsupported Redis numeric type %T", value)
	}
}
