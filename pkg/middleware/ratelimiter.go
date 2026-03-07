package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

type Result int

const (
	Allow Result = iota
	Skip
	Deny
)

type RateLimitingRule func(c echo.Context) (Result, error)

func UseRateLimiter(rules ...RateLimitingRule) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			for _, r := range rules {
				allowed, err := r(c)
				if allowed == Deny || err != nil {
					return echo.NewHTTPError(http.StatusTooManyRequests, "ratelimited")
				}
			}
			return next(c)
		}
	}
}

// tokenBucketScript atomically implements a token bucket in Redis.
//
// KEYS[1] = bucket key (e.g. "ratelimit:anon:1.2.3.4")
// ARGV[1] = rate      – tokens added per second (= limit / window.Seconds(), float)
// ARGV[2] = capacity  – maximum token count (= limit, float)
// ARGV[3] = now       – current Unix time as fractional seconds (float)
// ARGV[4] = ttl       – key TTL in seconds (integer)
//
// Returns 1 if the request is allowed (token consumed), 0 if denied.
var tokenBucketScript = redis.NewScript(`
local data        = redis.call('HMGET', KEYS[1], 'token_count', 'last_refill')
local token_count = tonumber(data[1])
local last_refill = tonumber(data[2])

local rate     = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local now      = tonumber(ARGV[3])
local ttl      = tonumber(ARGV[4])

-- first request: start with a full bucket
if token_count == nil then
	token_count = capacity
	last_refill = now
end

-- refill tokens based on elapsed time
local elapsed  = now - last_refill
local refilled = token_count + elapsed * rate
if refilled > capacity then
	refilled = capacity
end

-- deny if no tokens remain
if refilled < 1.0 then
	redis.call('HMSET', KEYS[1], 'token_count', refilled, 'last_refill', now)
	redis.call('EXPIRE', KEYS[1], ttl)
	return 0
end

-- consume one token and allow
redis.call('HMSET', KEYS[1], 'token_count', refilled - 1.0, 'last_refill', now)
redis.call('EXPIRE', KEYS[1], ttl)
return 1
`)

// tokenBucket runs the token bucket check atomically via a Redis Lua script.
// rate is tokens/second (= limit / window.Seconds()); capacity is the burst ceiling (= limit).
// Fails open on Redis errors so an outage doesn't block all traffic.
func tokenBucket(c echo.Context, rdb *redis.Client, key string, rate, capacity float64) (Result, error) {
	now := float64(time.Now().UnixNano()) / 1e9
	// capacity/rate == window in seconds; keep the key alive for 2 full windows
	ttl := int(capacity/rate) * 2

	res, err := tokenBucketScript.Run(
		c.Request().Context(),
		rdb,
		[]string{key},
		rate,
		capacity,
		now,
		ttl,
	).Int()
	if err != nil {
		return Allow, nil // fail open
	}
	if res == 0 {
		return Deny, nil
	}
	return Allow, nil
}

// LimitAnonUsers rate-limits unauthenticated requests by IP.
// Skips requests that carry a JWT (handled by the user rules).
// Example: LimitAnonUsers(rdb, 100, time.Hour)  →  100 req/hour per IP
func LimitAnonUsers(redisClient *redis.Client, limit int, window time.Duration) RateLimitingRule {
	rate := float64(limit) / window.Seconds()
	capacity := float64(limit)
	return func(c echo.Context) (Result, error) {
		if extractClaims(c) != nil {
			return Skip, nil
		}
		key := fmt.Sprintf("ratelimit:anon:%s", c.RealIP())
		return tokenBucket(c, redisClient, key, rate, capacity)
	}
}

// // LimitUsersByRoles rate-limits authenticated requests by username.
// // Skips anonymous requests and requests from users with one of the given roles.
// // Example: LimitUsersByRoles(rdb, 600, time.Hour, "Premium")  →  600 req/hour, skips Premium role
// func LimitUsersByRoles(redisClient *redis.Client, limit int, window time.Duration, roles ...string) RateLimitingRule {
// 	rate := float64(limit) / window.Seconds()
// 	capacity := float64(limit)
// 	rls := make(map[string]struct{}, len(roles))
// 	for _, r := range roles {
// 		rls[r] = struct{}{}
// 	}
// 	return func(c echo.Context) (Result, error) {
// 		claims := extractClaims(c)
// 		if claims == nil {
// 			return Skip, nil
// 		}
// 		if _, ok := rls[claims.Role]; ok {
// 			return Skip, nil
// 		}
// 		key := fmt.Sprintf("ratelimit:user:%s", claims.Username)
// 		return tokenBucket(c, redisClient, key, rate, capacity)
// 	}
// }
