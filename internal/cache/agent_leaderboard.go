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

// AgentLeaderboardLoader loads agent leaderboard rows from the database for a given coin.
type AgentLeaderboardLoader func(ctx context.Context, coinID int) ([]repository.AgentLeaderboardRow, error)

// AgentLeaderboardCache provides Redis-based caching with singleflight deduplication
// for agent leaderboard queries.
type AgentLeaderboardCache struct {
	redis  *redis.Client
	loader AgentLeaderboardLoader
	sf     singleflight.Group
	ttl    time.Duration
}

// NewAgentLeaderboardCache creates a new agent leaderboard cache.
func NewAgentLeaderboardCache(redisClient *redis.Client, loader AgentLeaderboardLoader, ttl time.Duration) *AgentLeaderboardCache {
	return &AgentLeaderboardCache{
		redis:  redisClient,
		loader: loader,
		ttl:    ttl,
	}
}

func agentLeaderboardKey(coinID int) string {
	return fmt.Sprintf("agent-leaderboard:%d", coinID)
}

// Get returns agent leaderboard rows for a coin.
// Checks Redis first, falls back to DB loader, caches the result.
func (c *AgentLeaderboardCache) Get(ctx context.Context, coinID int) ([]repository.AgentLeaderboardRow, error) {
	key := agentLeaderboardKey(coinID)

	val, err, _ := c.sf.Do(key, func() (interface{}, error) {
		// Check Redis
		if c.redis != nil {
			cached, redisErr := c.redis.Get(ctx, key).Bytes()
			if redisErr == nil {
				var rows []repository.AgentLeaderboardRow
				if json.Unmarshal(cached, &rows) == nil {
					return rows, nil
				}
			}
			if redisErr != nil && redisErr != redis.Nil {
				slog.WarnContext(ctx, "agent leaderboard cache: redis GET error", "key", key, "error", redisErr)
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
					slog.WarnContext(ctx, "agent leaderboard cache: redis SET error", "key", key, "error", setErr)
				}
			}
		}

		return rows, nil
	})

	if err != nil {
		return nil, err
	}

	return val.([]repository.AgentLeaderboardRow), nil
}
