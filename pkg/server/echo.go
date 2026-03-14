package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func Serve(e *echo.Echo, addr string) error {
	errCh := make(chan error, 1)
	go func() {
		if err := e.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err // startup failure (e.g. port already in use)
	case <-quit:
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return e.Shutdown(ctx)
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

// tokenBucket runs the token bucket check atomically via a Redis
// Lua script. rate is tokens/second (= limit / window.Seconds());
// capacity is the burst ceiling (= limit). Fails open on Redis
// returns true so an outage doesn't block all traffic.
func tokenBucketLimiter(
	c context.Context,
	rdb *redis.Client,
	key string,
	rate,
	capacity float64,
) bool {
	now := float64(time.Now().UnixNano()) / 1e9
	// capacity/rate == window in seconds; keep the key alive for 2 full windows
	ttl := int(capacity/rate) * 2

	res, err := tokenBucketScript.Run(
		c,
		rdb,
		[]string{key},
		rate,
		capacity,
		now,
		ttl,
	).Int()
	if err != nil {
		return true
	}
	if res == 0 {
		return false
	}
	return true
}

func Limit(
	redisClient *redis.Client,
	c context.Context,
	limit int,
	window time.Duration,
	user string,
	convoID string,
) bool {
	rate := float64(limit) / window.Seconds()
	capacity := float64(limit)
	key := fmt.Sprintf("ratelimit:%s:%s", user, convoID)
	return tokenBucketLimiter(c, redisClient, key, rate, capacity)
}
