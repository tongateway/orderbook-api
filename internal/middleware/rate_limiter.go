package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	defaultRateLimit = 1 // Default rate limit in requests per second
	rateLimitPrefix  = "rate_limit:"
)

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
			// If Redis error, log and allow request (fail open)
			slog.WarnContext(ctx, "Rate limiter Redis error",
				"error", err,
				"key", redisKey,
			)
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
