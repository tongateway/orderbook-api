package repository

import (
	dbmodels "api/internal/database/models"
	"api/internal/middleware"
	"context"
	"fmt"
	"time"
)

var allowedOrderSortColumns = map[string]string{
	"id":          "orders.id",
	"created_at":  "orders.created_at",
	"deployed_at": "orders.deployed_at",
	"status":      "orders.status",
	"type":        "orders.type",
	"amount":      "orders.amount",
	"price_rate":  "orders.price_rate",
}

// OrderFilters holds typed, validated filter parameters for order queries.
type OrderFilters struct {
	FromCoinID      *int64
	ToCoinID        *int64
	Status          *string
	MinAmount       *int64
	MaxAmount       *int64
	MinPriceRate    *int64
	MaxPriceRate    *int64
	MinSlippage     *int64
	MaxSlippage     *int64
	OwnerRawAddress *string
}

type OrderStats struct {
	Status string
	Count  int64
}

// DeployedTotalRow is one row of summed amount per coin for deployed orders
type DeployedTotalRow struct {
	CoinID      int
	Symbol      *string
	Name        *string
	TotalAmount int64
}

// OrderBookLevel represents an aggregated price level for the order book
type OrderBookLevel struct {
	PriceRate   int64 `json:"price_rate"`
	TotalAmount int64 `json:"total_amount"`
	OrderCount  int64 `json:"order_count"`
}

// TradingStatsRow represents one row of trading statistics grouped by status.
type TradingStatsRow struct {
	Status string
	Count  int64
	Volume int64
}

type OrderRepository interface {
	GetList(ctx context.Context, offset int, limit int, orderClauses []string, order string, filters OrderFilters) ([]dbmodels.Order, error)
	GetByID(ctx context.Context, id uint64) (*dbmodels.Order, error)
	GetStatsByWalletAddress(ctx context.Context, walletAddress string) ([]OrderStats, int64, error)
	GetDeployedTotalsByWalletAddress(ctx context.Context, walletAddress string) ([]DeployedTotalRow, error)
	GetOrderBook(ctx context.Context, fromCoinID, toCoinID int) ([]OrderBookLevel, error)
	GetTradingStats(ctx context.Context, fromCoinID, toCoinID int, since time.Time) ([]TradingStatsRow, error)
}

type orderRepository struct {
}

func NewOrderRepository() OrderRepository {
	return &orderRepository{}
}

func (r *orderRepository) GetList(ctx context.Context, offset int, limit int, orderClauses []string, order string, filters OrderFilters) ([]dbmodels.Order, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Validate order direction
	if order != "asc" && order != "desc" {
		order = "asc"
	}

	var orders []dbmodels.Order

	// Check if wallet join is needed
	needsWalletJoin := filters.OwnerRawAddress != nil

	dbq := session.WithContext(ctx)

	if needsWalletJoin {
		dbq = dbq.Joins("JOIN wallets ON orders.wallet_id = wallets.id")
	} else {
		dbq = dbq.Preload("Wallet")
	}

	// Always preload Vault
	dbq = dbq.Preload("Vault").Offset(offset).Limit(limit)

	// Apply filters using parameterized queries
	if filters.FromCoinID != nil {
		dbq = dbq.Where("from_coin_id = ?", *filters.FromCoinID)
	}
	if filters.ToCoinID != nil {
		dbq = dbq.Where("to_coin_id = ?", *filters.ToCoinID)
	}
	if filters.Status != nil {
		dbq = dbq.Where("status = ?", *filters.Status)
	}
	if filters.MinAmount != nil {
		dbq = dbq.Where("amount >= ?", *filters.MinAmount)
	}
	if filters.MaxAmount != nil {
		dbq = dbq.Where("amount <= ?", *filters.MaxAmount)
	}
	if filters.MinPriceRate != nil {
		dbq = dbq.Where("price_rate >= ?", *filters.MinPriceRate)
	}
	if filters.MaxPriceRate != nil {
		dbq = dbq.Where("price_rate <= ?", *filters.MaxPriceRate)
	}
	if filters.MinSlippage != nil {
		dbq = dbq.Where("slippage >= ?", *filters.MinSlippage)
	}
	if filters.MaxSlippage != nil {
		dbq = dbq.Where("slippage <= ?", *filters.MaxSlippage)
	}
	if filters.OwnerRawAddress != nil {
		dbq = dbq.Where("wallets.raw_address = ?", *filters.OwnerRawAddress)
	}

	// Apply validated sort columns
	for _, clause := range orderClauses {
		col, ok := allowedOrderSortColumns[clause]
		if !ok {
			return nil, fmt.Errorf("invalid sort column: %s", clause)
		}
		dbq = dbq.Order(col + " " + order)
	}

	stmt := dbq.Find(&orders)
	return orders, stmt.Error
}

// GetByID retrieves an order by ID from the database
// Uses database session from context for isolation
func (r *orderRepository) GetByID(ctx context.Context, id uint64) (*dbmodels.Order, error) {
	// Get database session for this request
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var order dbmodels.Order
	stmt := session.WithContext(ctx).Where("id = ?", id).First(&order)
	return &order, stmt.Error
}

// GetStatsByWalletAddress returns order counts grouped by status for the given wallet address
func (r *orderRepository) GetStatsByWalletAddress(ctx context.Context, walletAddress string) ([]OrderStats, int64, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, 0, err
	}

	var stats []OrderStats
	dbq := session.WithContext(ctx).Model(&dbmodels.Order{}).
		Joins("JOIN wallets ON orders.wallet_id = wallets.id").
		Where("wallets.raw_address = ?", walletAddress).
		Select("orders.status AS status, count(*) AS count").
		Group("orders.status")

	if err := dbq.Scan(&stats).Error; err != nil {
		return nil, 0, err
	}

	var total int64
	if err := session.WithContext(ctx).Model(&dbmodels.Order{}).
		Joins("JOIN wallets ON orders.wallet_id = wallets.id").
		Where("wallets.raw_address = ?", walletAddress).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	return stats, total, nil
}

// GetDeployedTotalsByWalletAddress returns sum of order amounts grouped by from_coin_id for deployed orders
func (r *orderRepository) GetDeployedTotalsByWalletAddress(ctx context.Context, walletAddress string) ([]DeployedTotalRow, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var rows []DeployedTotalRow
	err = session.WithContext(ctx).Model(&dbmodels.Order{}).
		Joins("JOIN wallets ON orders.wallet_id = wallets.id").
		Joins("LEFT JOIN coins ON orders.from_coin_id = coins.id").
		Where("wallets.raw_address = ? AND orders.status IN ('deployed', 'pending_match')", walletAddress).
		Select("COALESCE(orders.from_coin_id, 0) AS coin_id, COALESCE(coins.symbol, 'TON') AS symbol, COALESCE(coins.name, 'Toncoin') AS name, COALESCE(SUM(orders.amount), 0) AS total_amount").
		Group("COALESCE(orders.from_coin_id, 0), COALESCE(coins.symbol, 'TON'), COALESCE(coins.name, 'Toncoin')").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	return rows, nil
}

// GetOrderBook returns aggregated price levels for deployed orders of a specific pair.
// Levels are sorted by price_rate ASC.
// coinID = 0 means TON (from_coin_id IS NULL / to_coin_id IS NULL in DB).
func (r *orderRepository) GetOrderBook(ctx context.Context, fromCoinID, toCoinID int) ([]OrderBookLevel, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	dbq := session.WithContext(ctx).
		Model(&dbmodels.Order{}).
		Where("status = 'deployed'")

	// TON has NULL coin_id in orders table
	if fromCoinID == 0 {
		dbq = dbq.Where("from_coin_id IS NULL")
	} else {
		dbq = dbq.Where("from_coin_id = ?", fromCoinID)
	}

	if toCoinID == 0 {
		dbq = dbq.Where("to_coin_id IS NULL")
	} else {
		dbq = dbq.Where("to_coin_id = ?", toCoinID)
	}

	var levels []OrderBookLevel
	err = dbq.
		Select("price_rate, COALESCE(SUM(amount), 0) AS total_amount, COUNT(*) AS order_count").
		Group("price_rate").
		Order("price_rate ASC").
		Scan(&levels).Error
	if err != nil {
		return nil, err
	}

	return levels, nil
}

// GetTradingStats returns order counts and volumes grouped by status for a pair since a given time.
func (r *orderRepository) GetTradingStats(ctx context.Context, fromCoinID, toCoinID int, since time.Time) ([]TradingStatsRow, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	dbq := session.WithContext(ctx).
		Model(&dbmodels.Order{}).
		Where("created_at >= ?", since)

	if fromCoinID == 0 {
		dbq = dbq.Where("from_coin_id IS NULL")
	} else {
		dbq = dbq.Where("from_coin_id = ?", fromCoinID)
	}

	if toCoinID == 0 {
		dbq = dbq.Where("to_coin_id IS NULL")
	} else {
		dbq = dbq.Where("to_coin_id = ?", toCoinID)
	}

	var rows []TradingStatsRow
	err = dbq.
		Select("status, COUNT(*) AS count, COALESCE(SUM(initial_amount), 0) AS volume").
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	return rows, nil
}
