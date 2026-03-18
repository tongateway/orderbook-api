package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"api/internal/repository"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// TradingStatsLoader loads trading stats rows from the database for a given pair and time window.
type TradingStatsLoader func(ctx context.Context, fromCoinID, toCoinID int, since time.Time) ([]repository.TradingStatsRow, error)

// TradingStatsCache provides Redis-based caching with singleflight deduplication
// for trading stats queries.
type TradingStatsCache struct {
	redis  *redis.Client
	loader TradingStatsLoader
	sf     singleflight.Group
	ttl    time.Duration
}

// NewTradingStatsCache creates a new trading stats cache.
func NewTradingStatsCache(redisClient *redis.Client, loader TradingStatsLoader, ttl time.Duration) *TradingStatsCache {
	return &TradingStatsCache{
		redis:  redisClient,
		loader: loader,
		ttl:    ttl,
	}
}

func tradingStatsKey(fromCoinID, toCoinID int, period string) string {
	return fmt.Sprintf("trading-stats:%d:%d:%s", fromCoinID, toCoinID, period)
}

// Get returns trading stats rows for a pair and period.
// Checks Redis first, falls back to DB loader, caches the result.
func (c *TradingStatsCache) Get(ctx context.Context, fromCoinID, toCoinID int, period string, since time.Time) ([]repository.TradingStatsRow, error) {
	key := tradingStatsKey(fromCoinID, toCoinID, period)

	val, err, _ := c.sf.Do(key, func() (interface{}, error) {
		// Check Redis
		if c.redis != nil {
			cached, redisErr := c.redis.Get(ctx, key).Bytes()
			if redisErr == nil {
				var rows []repository.TradingStatsRow
				if json.Unmarshal(cached, &rows) == nil {
					return rows, nil
				}
			}
			if redisErr != nil && redisErr != redis.Nil {
				slog.WarnContext(ctx, "trading stats cache: redis GET error", "key", key, "error", redisErr)
			}
		}

		// Load from DB
		rows, dbErr := c.loader(ctx, fromCoinID, toCoinID, since)
		if dbErr != nil {
			return nil, dbErr
		}

		// Store in Redis
		if c.redis != nil {
			if data, marshalErr := json.Marshal(rows); marshalErr == nil {
				if setErr := c.redis.Set(ctx, key, data, c.ttl).Err(); setErr != nil {
					slog.WarnContext(ctx, "trading stats cache: redis SET error", "key", key, "error", setErr)
				}
			}
		}

		return rows, nil
	})

	if err != nil {
		return nil, err
	}

	return val.([]repository.TradingStatsRow), nil
}
