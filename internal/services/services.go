package services

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"time"

	"api/internal/cache"
	"api/internal/config"
	"api/internal/database"
	"api/internal/repository"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Services is a service locator that holds all application services and repositories
type Services struct {
	// Database connection pool
	DB *gorm.DB

	// Redis client
	Redis *redis.Client

	// Repositories
	CoinsRepo   repository.CoinsRepository
	OrderRepo   repository.OrderRepository
	VaultRepo   repository.VaultRepository
	APIKeysRepo repository.APIKeysRepository

	// Caches
	OrderBookCache         *cache.OrderBookCache
	TradingStatsCache      *cache.TradingStatsCache
	CoinPriceCache         *cache.CoinPriceCache
	AgentLeaderboardCache  *cache.AgentLeaderboardCache

	Repositories *repository.Repositories
}

// NewServices initializes all services, repositories, and database connections
func NewServices(cfg *config.Config) (*Services, error) {
	// Initialize database connection pool
	db, err := database.NewDatabase(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize Redis client
	redisOpts := &redis.Options{
		Addr:     cfg.Redis.Host + ":" + cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}
	if cfg.Redis.TLS {
		redisOpts.TLSConfig = &tls.Config{}
	}
	redisClient := redis.NewClient(redisOpts)

	// Test Redis connection (will be tested on first use, so we skip explicit ping here)
	// Redis client will automatically handle connection errors on first operation

	slog.Info("Redis connection established")

	// Initialize repositories (they will use DB session from context)
	coinsRepo := repository.NewCoinsRepository()
	orderRepo := repository.NewOrderRepository()
	vaultRepo := repository.NewVaultRepository()
	apiKeysRepo := repository.NewAPIKeysRepository()

	repositories := repository.InitRepositories()

	// Initialize order book cache: L1 = 1s in-memory, L2 = 5s Redis
	orderBookCache := cache.NewOrderBookCache(
		redisClient,
		orderRepo.GetOrderBook,
		1*time.Second,
		5*time.Second,
	)

	// Initialize trading stats cache: Redis 30s
	tradingStatsCache := cache.NewTradingStatsCache(
		redisClient,
		orderRepo.GetTradingStats,
		30*time.Second,
	)

	// Initialize coin price cache: Redis 30s
	coinPriceCache := cache.NewCoinPriceCache(
		redisClient,
		orderRepo.GetCoinPriceSummary,
		30*time.Second,
	)

	// Initialize agent leaderboard cache: Redis 30s
	agentLeaderboardCache := cache.NewAgentLeaderboardCache(
		redisClient,
		orderRepo.GetAgentLeaderboard,
		30*time.Second,
	)

	return &Services{
		DB:                db,
		Redis:             redisClient,
		CoinsRepo:         coinsRepo,
		OrderRepo:         orderRepo,
		VaultRepo:         vaultRepo,
		APIKeysRepo:       apiKeysRepo,
		OrderBookCache:    orderBookCache,
		TradingStatsCache: tradingStatsCache,
		CoinPriceCache:         coinPriceCache,
		AgentLeaderboardCache:  agentLeaderboardCache,
		Repositories:           repositories,
	}, nil
}

// Close closes all connections and cleans up resources
func (s *Services) Close() error {
	var errs []error

	if s.OrderBookCache != nil {
		s.OrderBookCache.Close()
	}

	if s.Redis != nil {
		if err := s.Redis.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close Redis: %w", err))
		}
	}

	if s.DB != nil {
		if err := database.CloseDatabase(s.DB); err != nil {
			errs = append(errs, fmt.Errorf("failed to close database: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing services: %v", errs)
	}

	return nil
}
