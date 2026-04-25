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
	MinPriceRate    *string
	MaxPriceRate    *string
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
	PriceRate   string `json:"price_rate"`
	TotalAmount int64  `json:"total_amount"`
	OrderCount  int64  `json:"order_count"`
}

// TradingStatsRow represents one row of trading statistics grouped by status.
type TradingStatsRow struct {
	Status string
	Count  int64
	Volume int64
}

// OrderBookFilter holds optional per-order filters applied before aggregation
// in GetOrderBook. Zero values mean "no filter".
type OrderBookFilter struct {
	// MinAmount filters orders where amount >= MinAmount (smallest from_coin units).
	// Used when from_coin is a stablecoin and we want USD-denominated filtering.
	MinAmount int64
	// MinTotalValue filters orders where amount * price_rate >= MinTotalValue
	// (as a numeric string, because the product can overflow int64).
	// Used when to_coin is a stablecoin.
	MinTotalValue string
}

// CoinPairPriceRow represents one row from the coin price summary query.
// Side is "ask" (orders selling the coin) or "bid" (orders buying the coin).
type CoinPairPriceRow struct {
	CounterCoinID int    `json:"counter_coin_id"`
	Side          string `json:"side"`
	BestPrice     string `json:"best_price"`
	OrderCount    int64  `json:"order_count"`
	TotalAmount   int64  `json:"total_amount"`
}

// AgentLeaderboardRow represents one agent's aggregated trading stats for a coin.
type AgentLeaderboardRow struct {
	RawAddress      string `json:"raw_address"`
	TotalOrders     int64  `json:"total_orders"`
	CompletedOrders int64  `json:"completed_orders"`
	DeployedOrders  int64  `json:"deployed_orders"`
	CompletedVolume int64  `json:"completed_volume"`
	BuyVolume       int64  `json:"buy_volume"`
	SellVolume      int64  `json:"sell_volume"`
}

// BatchContextResult holds orders and deployed totals for a single wallet address.
type BatchContextResult struct {
	Orders         []dbmodels.Order  `json:"orders"`
	DeployedTotals []DeployedTotalRow `json:"deployed_totals"`
}

// CandleRow represents a single OHLCV candle built from completed orders.
type CandleRow struct {
	BucketTs int64  `json:"bucket_ts"`
	Open     string `json:"open"`
	High     string `json:"high"`
	Low      string `json:"low"`
	Close    string `json:"close"`
	Volume   int64  `json:"volume"`
}

type OrderRepository interface {
	GetList(ctx context.Context, offset int, limit int, orderClauses []string, order string, filters OrderFilters) ([]dbmodels.Order, error)
	GetByID(ctx context.Context, id uint64) (*dbmodels.Order, error)
	GetStatsByWalletAddress(ctx context.Context, walletAddress string) ([]OrderStats, int64, error)
	GetDeployedTotalsByWalletAddress(ctx context.Context, walletAddress string) ([]DeployedTotalRow, error)
	GetBatchContext(ctx context.Context, walletAddresses []string, status string) (map[string]*BatchContextResult, error)
	GetOrderBook(ctx context.Context, fromCoinID, toCoinID int, filter OrderBookFilter) ([]OrderBookLevel, error)
	GetTradingStats(ctx context.Context, fromCoinID, toCoinID int, since time.Time) ([]TradingStatsRow, error)
	GetCoinPriceSummary(ctx context.Context, coinID int) ([]CoinPairPriceRow, error)
	GetAgentLeaderboard(ctx context.Context, coinID int) ([]AgentLeaderboardRow, error)
	GetCandles(ctx context.Context, fromCoinID, toCoinID int, intervalSec int64, since time.Time, limit int) ([]CandleRow, error)
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
		dbq = dbq.Where("price_rate >= ?::numeric", *filters.MinPriceRate)
	}
	if filters.MaxPriceRate != nil {
		dbq = dbq.Where("price_rate <= ?::numeric", *filters.MaxPriceRate)
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
// Only orders with amount > 0 are included (depleted orders, fully matched but
// not yet marked closed, are excluded to keep the book clean).
func (r *orderRepository) GetOrderBook(ctx context.Context, fromCoinID, toCoinID int, filter OrderBookFilter) ([]OrderBookLevel, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	dbq := session.WithContext(ctx).
		Model(&dbmodels.Order{}).
		Where("status = 'deployed'").
		Where("amount > 0")

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

	// USD-denominated filter applied BEFORE grouping so each individual order
	// meets the threshold.
	if filter.MinAmount > 0 {
		dbq = dbq.Where("amount >= ?", filter.MinAmount)
	}
	if filter.MinTotalValue != "" {
		// price_rate is numeric; cast the bound to numeric to avoid int overflow.
		dbq = dbq.Where("amount * price_rate >= CAST(? AS numeric)", filter.MinTotalValue)
	}

	var levels []OrderBookLevel
	err = dbq.
		Select("price_rate, COALESCE(SUM(amount), 0) AS total_amount, COUNT(*) AS order_count").
		Group("price_rate").
		Having("COALESCE(SUM(amount), 0) > 0").
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

// GetCoinPriceSummary returns aggregated order book summary for all trading pairs
// involving the given coin. Returns ask and bid rows for each counter-party coin.
// coinID = 0 means TON (NULL in DB).
func (r *orderRepository) GetCoinPriceSummary(ctx context.Context, coinID int) ([]CoinPairPriceRow, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var rows []CoinPairPriceRow

	if coinID == 0 {
		query := `
			SELECT COALESCE(to_coin_id, 0) AS counter_coin_id, 'ask' AS side,
				MIN(price_rate) AS best_price, COUNT(*) AS order_count,
				COALESCE(SUM(amount), 0) AS total_amount
			FROM orders
			WHERE status = 'deployed' AND from_coin_id IS NULL
			GROUP BY COALESCE(to_coin_id, 0)
			UNION ALL
			SELECT COALESCE(from_coin_id, 0) AS counter_coin_id, 'bid' AS side,
				MAX(price_rate) AS best_price, COUNT(*) AS order_count,
				COALESCE(SUM(amount), 0) AS total_amount
			FROM orders
			WHERE status = 'deployed' AND to_coin_id IS NULL
			GROUP BY COALESCE(from_coin_id, 0)
		`
		err = session.WithContext(ctx).Raw(query).Scan(&rows).Error
	} else {
		query := `
			SELECT COALESCE(to_coin_id, 0) AS counter_coin_id, 'ask' AS side,
				MIN(price_rate) AS best_price, COUNT(*) AS order_count,
				COALESCE(SUM(amount), 0) AS total_amount
			FROM orders
			WHERE status = 'deployed' AND from_coin_id = $1
			GROUP BY COALESCE(to_coin_id, 0)
			UNION ALL
			SELECT COALESCE(from_coin_id, 0) AS counter_coin_id, 'bid' AS side,
				MAX(price_rate) AS best_price, COUNT(*) AS order_count,
				COALESCE(SUM(amount), 0) AS total_amount
			FROM orders
			WHERE status = 'deployed' AND to_coin_id = $2
			GROUP BY COALESCE(from_coin_id, 0)
		`
		err = session.WithContext(ctx).Raw(query, coinID, coinID).Scan(&rows).Error
	}

	if err != nil {
		return nil, err
	}

	return rows, nil
}

// GetAgentLeaderboard returns aggregated trading stats per agent (wallet) for orders involving the given coin.
// coinID = 0 means TON (NULL in DB).
func (r *orderRepository) GetAgentLeaderboard(ctx context.Context, coinID int) ([]AgentLeaderboardRow, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var rows []AgentLeaderboardRow

	if coinID == 0 {
		query := `
			SELECT
				w.raw_address,
				COUNT(*) AS total_orders,
				COUNT(*) FILTER (WHERE o.status = 'completed') AS completed_orders,
				COUNT(*) FILTER (WHERE o.status = 'deployed') AS deployed_orders,
				COALESCE(SUM(o.initial_amount) FILTER (WHERE o.status = 'completed'), 0) AS completed_volume,
				COALESCE(SUM(o.initial_amount) FILTER (WHERE o.status = 'completed' AND o.to_coin_id IS NULL), 0) AS buy_volume,
				COALESCE(SUM(o.initial_amount) FILTER (WHERE o.status = 'completed' AND o.from_coin_id IS NULL), 0) AS sell_volume
			FROM orders o
			JOIN wallets w ON o.wallet_id = w.id
			WHERE (o.from_coin_id IS NULL OR o.to_coin_id IS NULL)
			GROUP BY w.raw_address
			ORDER BY completed_volume DESC, completed_orders DESC
			-- Audit 05-M1: hard cap to bound memory + DB load even if cache TTL drives a fresh scan.
			-- Handler still applies its own pagination on the cached slice.
			LIMIT 1000
		`
		err = session.WithContext(ctx).Raw(query).Scan(&rows).Error
	} else {
		query := `
			SELECT
				w.raw_address,
				COUNT(*) AS total_orders,
				COUNT(*) FILTER (WHERE o.status = 'completed') AS completed_orders,
				COUNT(*) FILTER (WHERE o.status = 'deployed') AS deployed_orders,
				COALESCE(SUM(o.initial_amount) FILTER (WHERE o.status = 'completed'), 0) AS completed_volume,
				COALESCE(SUM(o.initial_amount) FILTER (WHERE o.status = 'completed' AND o.to_coin_id = $1), 0) AS buy_volume,
				COALESCE(SUM(o.initial_amount) FILTER (WHERE o.status = 'completed' AND o.from_coin_id = $2), 0) AS sell_volume
			FROM orders o
			JOIN wallets w ON o.wallet_id = w.id
			WHERE (o.from_coin_id = $3 OR o.to_coin_id = $4)
			GROUP BY w.raw_address
			ORDER BY completed_volume DESC, completed_orders DESC
			-- Audit 05-M1: hard cap to bound memory + DB load even if cache TTL drives a fresh scan.
			-- Handler still applies its own pagination on the cached slice.
			LIMIT 1000
		`
		err = session.WithContext(ctx).Raw(query, coinID, coinID, coinID, coinID).Scan(&rows).Error
	}

	if err != nil {
		return nil, err
	}

	return rows, nil
}

// GetCandles builds OHLCV candles from completed/closed orders for a trading pair.
// intervalSec is the candle width in seconds (e.g. 60 for 1m, 3600 for 1h).
// Candles are built by bucketing order.created_at into time windows and aggregating price_rate / initial_amount.
func (r *orderRepository) GetCandles(ctx context.Context, fromCoinID, toCoinID int, intervalSec int64, since time.Time, limit int) ([]CandleRow, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if limit <= 0 || limit > 1000 {
		limit = 500
	}

	// Build parameterized query. The coin conditions use IS NULL for TON (id=0)
	// or parameterized values for jettons. intervalSec and limit are safe int types
	// validated above, so they can be interpolated directly.
	buildCoinCond := func(col string, coinID int, paramIdx *int, args *[]interface{}) string {
		if coinID == 0 {
			return col + " IS NULL"
		}
		*paramIdx++
		*args = append(*args, coinID)
		return fmt.Sprintf("%s = $%d", col, *paramIdx)
	}

	paramIdx := 0
	var args []interface{}
	fromCond := buildCoinCond("from_coin_id", fromCoinID, &paramIdx, &args)
	toCond := buildCoinCond("to_coin_id", toCoinID, &paramIdx, &args)

	paramIdx++
	args = append(args, since)
	sinceParam := fmt.Sprintf("$%d", paramIdx)

	paramIdx++
	args = append(args, intervalSec)
	intervalParam := fmt.Sprintf("$%d", paramIdx)

	paramIdx++
	args = append(args, limit)
	limitParam := fmt.Sprintf("$%d", paramIdx)

	query := fmt.Sprintf(`
		SELECT
			(floor(extract(epoch FROM created_at) / %s) * %s)::bigint AS bucket_ts,
			(array_agg(price_rate ORDER BY created_at ASC))[1]::text AS open,
			MAX(price_rate)::text AS high,
			MIN(price_rate)::text AS low,
			(array_agg(price_rate ORDER BY created_at DESC))[1]::text AS close,
			COALESCE(SUM(initial_amount), 0) AS volume
		FROM orders
		WHERE status IN ('completed', 'closed')
		  AND %s AND %s
		  AND created_at >= %s
		  AND price_rate IS NOT NULL
		GROUP BY bucket_ts
		ORDER BY bucket_ts ASC
		LIMIT %s
	`, intervalParam, intervalParam, fromCond, toCond, sinceParam, limitParam)

	var rows []CandleRow
	err = session.WithContext(ctx).Raw(query, args...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	return rows, nil
}

// GetBatchContext fetches orders and deployed totals for multiple wallet addresses in bulk.
// It executes two queries (orders + deployed totals) using IN clauses instead of N per-wallet queries.
func (r *orderRepository) GetBatchContext(ctx context.Context, walletAddresses []string, status string) (map[string]*BatchContextResult, error) {
	if len(walletAddresses) == 0 {
		return map[string]*BatchContextResult{}, nil
	}

	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Initialize result map for all requested addresses
	results := make(map[string]*BatchContextResult, len(walletAddresses))
	for _, addr := range walletAddresses {
		results[addr] = &BatchContextResult{
			Orders:         []dbmodels.Order{},
			DeployedTotals: []DeployedTotalRow{},
		}
	}

	// --- Query 1: Fetch orders for all wallets in one query ---
	var orders []dbmodels.Order
	dbq := session.WithContext(ctx).
		Joins("JOIN wallets ON orders.wallet_id = wallets.id").
		Preload("Vault").
		Where("wallets.raw_address IN ?", walletAddresses)

	if status != "" {
		dbq = dbq.Where("orders.status = ?", status)
	}

	if err := dbq.Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("batch orders query failed: %w", err)
	}

	// We need the wallet raw_address for each order to group them.
	// Since the JOIN doesn't populate the Wallet relation, query the wallet mapping.
	walletIDToAddr := make(map[int]string)
	if len(orders) > 0 {
		walletIDs := make([]int, 0, len(orders))
		seen := make(map[int]bool)
		for _, o := range orders {
			if o.WalletID != nil && !seen[*o.WalletID] {
				walletIDs = append(walletIDs, *o.WalletID)
				seen[*o.WalletID] = true
			}
		}

		type walletRow struct {
			ID         int    `gorm:"column:id"`
			RawAddress string `gorm:"column:raw_address"`
		}
		var wallets []walletRow
		if err := session.WithContext(ctx).Table("wallets").
			Select("id, raw_address").
			Where("id IN ?", walletIDs).
			Scan(&wallets).Error; err != nil {
			return nil, fmt.Errorf("batch wallet lookup failed: %w", err)
		}
		for _, w := range wallets {
			walletIDToAddr[w.ID] = w.RawAddress
		}
	}

	// Group orders by wallet address
	for _, o := range orders {
		if o.WalletID == nil {
			continue
		}
		addr, ok := walletIDToAddr[*o.WalletID]
		if !ok {
			continue
		}
		if r, ok := results[addr]; ok {
			r.Orders = append(r.Orders, o)
		}
	}

	// --- Query 2: Fetch deployed totals for all wallets in one query ---
	var totalRows []struct {
		RawAddress  string  `gorm:"column:raw_address"`
		CoinID      int     `gorm:"column:coin_id"`
		Symbol      *string `gorm:"column:symbol"`
		Name        *string `gorm:"column:name"`
		TotalAmount int64   `gorm:"column:total_amount"`
	}
	err = session.WithContext(ctx).Model(&dbmodels.Order{}).
		Joins("JOIN wallets ON orders.wallet_id = wallets.id").
		Joins("LEFT JOIN coins ON orders.from_coin_id = coins.id").
		Where("wallets.raw_address IN ? AND orders.status IN ('deployed', 'pending_match')", walletAddresses).
		Select("wallets.raw_address AS raw_address, COALESCE(orders.from_coin_id, 0) AS coin_id, COALESCE(coins.symbol, 'TON') AS symbol, COALESCE(coins.name, 'Toncoin') AS name, COALESCE(SUM(orders.amount), 0) AS total_amount").
		Group("wallets.raw_address, COALESCE(orders.from_coin_id, 0), COALESCE(coins.symbol, 'TON'), COALESCE(coins.name, 'Toncoin')").
		Scan(&totalRows).Error
	if err != nil {
		return nil, fmt.Errorf("batch deployed totals query failed: %w", err)
	}

	// Group deployed totals by wallet address
	for _, row := range totalRows {
		if r, ok := results[row.RawAddress]; ok {
			r.DeployedTotals = append(r.DeployedTotals, DeployedTotalRow{
				CoinID:      row.CoinID,
				Symbol:      row.Symbol,
				Name:        row.Name,
				TotalAmount: row.TotalAmount,
			})
		}
	}

	return results, nil
}
