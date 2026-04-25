package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	defaultRateLimit = 1 // Default rate limit in requests per second
	rateLimitPrefix  = "rate_limit:"

	// Audit 05-H5: server-side hard cap on per-API-key rate limits.
	// Without this, a misconfigured key with rate_limit=10000 in DB could
	// effectively bypass protection against authenticated DoS.
	maxAllowedRateLimit = 200

	// Audit 05-H4: fail-closed threshold. Rate limiter shouldn't allow
	// unlimited traffic if Redis breaks; in-memory fallback counter trips
	// to a strict per-IP limit during outage.
	failoverPerIPRPS = 10
)

// inMemoryFailover is a tiny per-instance counter used only when Redis is
// unreachable. Keys auto-expire via the cleanup goroutine.
var inMemoryFailover = struct {
	m map[string]*failoverEntry
}{m: make(map[string]*failoverEntry)}

type failoverEntry struct {
	count   int
	resetAt time.Time
}

// RateLimiter creates a rate limiter middleware using Redis.
// Authenticated requests use the per-key rate limit stored in the API key record.
// Anonymous requests (no token) are limited to anonymousRPS per IP address.
func RateLimiter(redisClient *redis.Client, defaultRPS int, anonymousRPS int, window time.Duration) gin.HandlerFunc {
	if defaultRPS <= 0 {
		defaultRPS = defaultRateLimit
	}
	if anonymousRPS <= 0 {
		anonymousRPS = defaultRateLimit
	}

	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Determine rate limit from context or use default
		var rateLimit int
		var key string

		// Try to get API key rate limit from context (set by APIKeyAuth middleware)
		if apiKeyRateLimit, exists := c.Get("api_key_rate_limit"); exists {
			if rl, ok := apiKeyRateLimit.(int); ok && rl > 0 {
				// Audit 05-H5: clamp to server-side max even if DB stores
				// a higher value.
				if rl > maxAllowedRateLimit {
					rl = maxAllowedRateLimit
				}
				rateLimit = rl
				// Get API key hash for Redis key
				if apiKeyHash, exists := c.Get("api_key_hash"); exists {
					if hash, ok := apiKeyHash.(string); ok && hash != "" {
						key = rateLimitPrefix + "api_key:" + hash
					}
				}
			}
		}

		// Fallback to anonymous rate limit and IP address
		if key == "" {
			rateLimit = anonymousRPS
			key = rateLimitPrefix + "ip:" + c.ClientIP()
		}

		// Use sliding window rate limiting with Redis
		now := time.Now()
		windowStart := now.Truncate(window)

		// Create Redis key with window timestamp
		redisKey := fmt.Sprintf("%s:%d", key, windowStart.Unix())

		// Increment counter and set expiration
		count, err := redisClient.Incr(ctx, redisKey).Result()
		if err != nil {
			// Audit 05-H4: when Redis is unreachable, fall over to a tiny
			// in-memory counter per IP. Strictly bounded — under outage we
			// drop into severe rate-limit rather than open the floodgates.
			slog.WarnContext(ctx, "Rate limiter Redis error — using in-memory fallback",
				"error", err, "key", redisKey,
			)
			if !allowFailover(c.ClientIP(), failoverPerIPRPS, window) {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error": "Rate limit exceeded (degraded mode)",
				})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		// Set expiration on first increment
		if count == 1 {
			redisClient.Expire(context.Background(), redisKey, window+time.Second)
		}

		// Check if limit exceeded
		if count > int64(rateLimit) {
			// Calculate retry after time
			retryAfter := windowStart.Add(window).Sub(now).Seconds()
			if retryAfter < 0 {
				retryAfter = 0
			}

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Too many requests",
				"message":     fmt.Sprintf("Rate limit exceeded. Limit: %d requests per %v", rateLimit, window),
				"retry_after": retryAfter,
			})
			c.Abort()
			return
		}

		// Add rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(rateLimit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(int64(rateLimit)-count, 10))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(windowStart.Add(window).Unix(), 10))

		c.Next()
	}
}

// allowFailover is the in-memory fallback used by RateLimiter when Redis
// is unavailable. Strict per-IP cap; entries auto-expire on next access.
// Audit 05-H4.
var failoverMu sync.Mutex

func allowFailover(ip string, maxReq int, window time.Duration) bool {
	failoverMu.Lock()
	defer failoverMu.Unlock()
	now := time.Now()
	e, ok := inMemoryFailover.m[ip]
	if !ok || now.After(e.resetAt) {
		inMemoryFailover.m[ip] = &failoverEntry{count: 1, resetAt: now.Add(window)}
		return true
	}
	e.count++
	return e.count <= maxReq
}
