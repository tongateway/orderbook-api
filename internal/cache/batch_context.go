package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"api/internal/repository"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// BatchContextLoader is a function that loads batch context from the database.
type BatchContextLoader func(ctx context.Context, walletAddresses []string, status string) (map[string]*repository.BatchContextResult, error)

type batchContextEntry struct {
	data      map[string]*repository.BatchContextResult
	expiresAt time.Time
}

// BatchContextCache provides a two-level cache (in-memory L1 + Redis L2) with singleflight
// deduplication for batch order context queries.
type BatchContextCache struct {
	redis  *redis.Client
	loader BatchContextLoader
	sf     singleflight.Group

	mu     sync.RWMutex
	memory map[string]*batchContextEntry

	l1TTL time.Duration // in-memory TTL
	l2TTL time.Duration // Redis TTL

	done chan struct{}
}

// NewBatchContextCache creates a new batch context cache.
//   - redisClient: Redis connection (may be nil - Redis layer will be skipped)
//   - loader: function that fetches data from DB
//   - l1TTL: in-memory cache TTL (e.g. 5s)
//   - l2TTL: Redis cache TTL (e.g. 15s)
func NewBatchContextCache(redisClient *redis.Client, loader BatchContextLoader, l1TTL, l2TTL time.Duration) *BatchContextCache {
	c := &BatchContextCache{
		redis:  redisClient,
		loader: loader,
		memory: make(map[string]*batchContextEntry),
		l1TTL:  l1TTL,
		l2TTL:  l2TTL,
		done:   make(chan struct{}),
	}
	go c.cleanup()
	return c
}

// Close stops the background cleanup goroutine.
func (c *BatchContextCache) Close() {
	close(c.done)
}

// cleanup periodically removes expired L1 entries to prevent unbounded memory growth.
func (c *BatchContextCache) cleanup() {
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

// batchContextKey produces a deterministic cache key from sorted wallet addresses and status.
func batchContextKey(walletAddresses []string, status string) string {
	sorted := make([]string, len(walletAddresses))
	copy(sorted, walletAddresses)
	sort.Strings(sorted)
	return fmt.Sprintf("batch_ctx:%s:%s", status, strings.Join(sorted, ","))
}

// Get returns batch context for the given wallet addresses and status.
// It checks L1 (in-memory) -> L2 (Redis) -> DB, using singleflight to deduplicate
// concurrent requests for the same key.
func (c *BatchContextCache) Get(ctx context.Context, walletAddresses []string, status string) (map[string]*repository.BatchContextResult, error) {
	key := batchContextKey(walletAddresses, status)

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
				var result map[string]*repository.BatchContextResult
				if json.Unmarshal(cached, &result) == nil {
					c.setL1(key, result)
					return result, nil
				}
			}
			if redisErr != nil && redisErr != redis.Nil {
				slog.WarnContext(ctx, "batch context cache: redis GET error", "key", key, "error", redisErr)
			}
		}

		// L3: Database
		result, dbErr := c.loader(ctx, walletAddresses, status)
		if dbErr != nil {
			return nil, dbErr
		}

		// Store in L2 (Redis) - non-fatal on error
		if c.redis != nil {
			if data, marshalErr := json.Marshal(result); marshalErr == nil {
				if setErr := c.redis.Set(ctx, key, data, c.l2TTL).Err(); setErr != nil {
					slog.WarnContext(ctx, "batch context cache: redis SET error", "key", key, "error", setErr)
				}
			}
		}

		// Store in L1
		c.setL1(key, result)

		return result, nil
	})

	if err != nil {
		return nil, err
	}

	return val.(map[string]*repository.BatchContextResult), nil
}

// getL1 returns cached data if the entry exists and is not expired.
func (c *BatchContextCache) getL1(key string) (map[string]*repository.BatchContextResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.memory[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

// setL1 stores data in the in-memory cache.
func (c *BatchContextCache) setL1(key string, data map[string]*repository.BatchContextResult) {
	c.mu.Lock()
	c.memory[key] = &batchContextEntry{
		data:      data,
		expiresAt: time.Now().Add(c.l1TTL),
	}
	c.mu.Unlock()
}
