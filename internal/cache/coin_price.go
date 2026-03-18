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

// CoinPriceLoader loads coin price summary rows from the database for a given coin.
type CoinPriceLoader func(ctx context.Context, coinID int) ([]repository.CoinPairPriceRow, error)

// CoinPriceCache provides Redis-based caching with singleflight deduplication
// for coin price summary queries.
type CoinPriceCache struct {
	redis  *redis.Client
	loader CoinPriceLoader
	sf     singleflight.Group
	ttl    time.Duration
}

// NewCoinPriceCache creates a new coin price cache.
func NewCoinPriceCache(redisClient *redis.Client, loader CoinPriceLoader, ttl time.Duration) *CoinPriceCache {
	return &CoinPriceCache{
		redis:  redisClient,
		loader: loader,
		ttl:    ttl,
	}
}

func coinPriceKey(coinID int) string {
	return fmt.Sprintf("coin-price:%d", coinID)
}

// Get returns coin price summary rows for a given coin.
// Checks Redis first, falls back to DB loader, caches the result.
func (c *CoinPriceCache) Get(ctx context.Context, coinID int) ([]repository.CoinPairPriceRow, error) {
	key := coinPriceKey(coinID)

	val, err, _ := c.sf.Do(key, func() (interface{}, error) {
		// Check Redis
		if c.redis != nil {
			cached, redisErr := c.redis.Get(ctx, key).Bytes()
			if redisErr == nil {
				var rows []repository.CoinPairPriceRow
				if json.Unmarshal(cached, &rows) == nil {
					return rows, nil
				}
			}
			if redisErr != nil && redisErr != redis.Nil {
				slog.WarnContext(ctx, "coin price cache: redis GET error", "key", key, "error", redisErr)
			}
		}

		// Load from DB
		rows, dbErr := c.loader(ctx, coinID)
		if dbErr != nil {
			return nil, dbErr
		}

		// Store in Redis
		if c.redis != nil {
			if data, marshalErr := json.Marshal(rows); marshalErr == nil {
				if setErr := c.redis.Set(ctx, key, data, c.ttl).Err(); setErr != nil {
					slog.WarnContext(ctx, "coin price cache: redis SET error", "key", key, "error", setErr)
				}
			}
		}

		return rows, nil
	})

	if err != nil {
		return nil, err
	}

	return val.([]repository.CoinPairPriceRow), nil
}
