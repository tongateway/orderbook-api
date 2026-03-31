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

// CandlesLoader loads candle rows from the database for a given pair, interval, and time window.
type CandlesLoader func(ctx context.Context, fromCoinID, toCoinID int, intervalSec int64, since time.Time, limit int) ([]repository.CandleRow, error)

// CandlesCache provides Redis-based caching with singleflight deduplication for candle queries.
type CandlesCache struct {
	redis  *redis.Client
	loader CandlesLoader
	sf     singleflight.Group
	ttl    time.Duration
}

// NewCandlesCache creates a new candles cache.
func NewCandlesCache(redisClient *redis.Client, loader CandlesLoader, ttl time.Duration) *CandlesCache {
	return &CandlesCache{
		redis:  redisClient,
		loader: loader,
		ttl:    ttl,
	}
}

func candlesKey(fromCoinID, toCoinID int, intervalSec int64, sinceUnix int64) string {
	return fmt.Sprintf("candles:%d:%d:%d:%d", fromCoinID, toCoinID, intervalSec, sinceUnix)
}

// Get returns candle rows for a pair, interval, and time window.
func (c *CandlesCache) Get(ctx context.Context, fromCoinID, toCoinID int, intervalSec int64, since time.Time, limit int) ([]repository.CandleRow, error) {
	key := candlesKey(fromCoinID, toCoinID, intervalSec, since.Unix())

	val, err, _ := c.sf.Do(key, func() (interface{}, error) {
		// Check Redis
		if c.redis != nil {
			cached, redisErr := c.redis.Get(ctx, key).Bytes()
			if redisErr == nil {
				var rows []repository.CandleRow
				if json.Unmarshal(cached, &rows) == nil {
					return rows, nil
				}
			}
			if redisErr != nil && redisErr != redis.Nil {
				slog.WarnContext(ctx, "candles cache: redis GET error", "key", key, "error", redisErr)
			}
		}

		// Load from DB
		rows, dbErr := c.loader(ctx, fromCoinID, toCoinID, intervalSec, since, limit)
		if dbErr != nil {
			return nil, dbErr
		}

		// Store in Redis
		if c.redis != nil {
			if data, marshalErr := json.Marshal(rows); marshalErr == nil {
				if setErr := c.redis.Set(ctx, key, data, c.ttl).Err(); setErr != nil {
					slog.WarnContext(ctx, "candles cache: redis SET error", "key", key, "error", setErr)
				}
			}
		}

		return rows, nil
	})

	if err != nil {
		return nil, err
	}

	return val.([]repository.CandleRow), nil
}
