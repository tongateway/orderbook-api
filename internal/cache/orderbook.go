package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"api/internal/repository"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// OrderBookLoader is a function that loads order book levels from the database.
type OrderBookLoader func(ctx context.Context, fromCoinID, toCoinID int) ([]repository.OrderBookLevel, error)

type cacheEntry struct {
	data      []repository.OrderBookLevel
	expiresAt time.Time
}

// OrderBookCache provides a two-level cache (in-memory L1 + Redis L2) with singleflight
// deduplication for order book queries.
type OrderBookCache struct {
	redis  *redis.Client
	loader OrderBookLoader
	sf     singleflight.Group

	mu     sync.RWMutex
	memory map[string]*cacheEntry

	l1TTL time.Duration // in-memory TTL
	l2TTL time.Duration // Redis TTL

	done chan struct{}
}

// NewOrderBookCache creates a new order book cache.
//   - redisClient: Redis connection (may be nil — Redis layer will be skipped)
//   - loader: function that fetches data from DB
//   - l1TTL: in-memory cache TTL (e.g. 1s)
//   - l2TTL: Redis cache TTL (e.g. 5s)
func NewOrderBookCache(redisClient *redis.Client, loader OrderBookLoader, l1TTL, l2TTL time.Duration) *OrderBookCache {
	c := &OrderBookCache{
		redis:  redisClient,
		loader: loader,
		memory: make(map[string]*cacheEntry),
		l1TTL:  l1TTL,
		l2TTL:  l2TTL,
		done:   make(chan struct{}),
	}
	go c.cleanup()
	return c
}

// Close stops the background cleanup goroutine.
func (c *OrderBookCache) Close() {
	close(c.done)
}

// cleanup periodically removes expired L1 entries to prevent unbounded memory growth.
func (c *OrderBookCache) cleanup() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			now := time.Now()
			c.mu.Lock()
			for k, entry := range c.memory {
				if now.After(entry.expiresAt) {
					delete(c.memory, k)
				}
			}
			c.mu.Unlock()
		}
	}
}

func cacheKey(fromCoinID, toCoinID int) string {
	return fmt.Sprintf("orderbook:%d:%d", fromCoinID, toCoinID)
}

// Get returns aggregated order book levels for a trading pair direction.
// It checks L1 (in-memory) → L2 (Redis) → DB, using singleflight to deduplicate
// concurrent requests for the same key.
// Returned slice is a safe copy that the caller can freely modify.
func (c *OrderBookCache) Get(ctx context.Context, fromCoinID, toCoinID int) ([]repository.OrderBookLevel, error) {
	key := cacheKey(fromCoinID, toCoinID)

	// L1: in-memory check
	if data, ok := c.getL1(key); ok {
		return data, nil
	}

	// singleflight: only one goroutine loads data; others wait for the result
	val, err, _ := c.sf.Do(key, func() (interface{}, error) {
		// Double-check L1 (another goroutine may have populated it)
		if data, ok := c.getL1(key); ok {
			return data, nil
		}

		// L2: Redis check
		if c.redis != nil {
			cached, redisErr := c.redis.Get(ctx, key).Bytes()
			if redisErr == nil {
				var levels []repository.OrderBookLevel
				if json.Unmarshal(cached, &levels) == nil {
					c.setL1(key, levels)
					return levels, nil
				}
			}
			if redisErr != nil && redisErr != redis.Nil {
				slog.WarnContext(ctx, "orderbook cache: redis GET error", "key", key, "error", redisErr)
			}
		}

		// L3: Database
		levels, dbErr := c.loader(ctx, fromCoinID, toCoinID)
		if dbErr != nil {
			return nil, dbErr
		}

		// Store in L2 (Redis) — non-fatal on error
		if c.redis != nil {
			if data, marshalErr := json.Marshal(levels); marshalErr == nil {
				if setErr := c.redis.Set(ctx, key, data, c.l2TTL).Err(); setErr != nil {
					slog.WarnContext(ctx, "orderbook cache: redis SET error", "key", key, "error", setErr)
				}
			}
		}

		// Store in L1
		c.setL1(key, levels)

		return levels, nil
	})

	if err != nil {
		return nil, err
	}

	// Return a copy so callers can safely mutate (e.g. reverse for bids)
	levels := val.([]repository.OrderBookLevel)
	return copyLevels(levels), nil
}

// getL1 returns a copy of cached levels if the entry exists and is not expired.
func (c *OrderBookCache) getL1(key string) ([]repository.OrderBookLevel, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.memory[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return copyLevels(entry.data), true
}

// setL1 stores a copy of levels in the in-memory cache.
func (c *OrderBookCache) setL1(key string, levels []repository.OrderBookLevel) {
	c.mu.Lock()
	c.memory[key] = &cacheEntry{
		data:      copyLevels(levels),
		expiresAt: time.Now().Add(c.l1TTL),
	}
	c.mu.Unlock()
}

// copyLevels creates a deep copy of the levels slice.
func copyLevels(src []repository.OrderBookLevel) []repository.OrderBookLevel {
	if src == nil {
		return []repository.OrderBookLevel{}
	}
	dst := make([]repository.OrderBookLevel, len(src))
	copy(dst, src)
	return dst
}
