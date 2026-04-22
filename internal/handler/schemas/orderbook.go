package schemas

// OrderBookRequestHTTP represents query parameters for the order book endpoint.
// Pair can be specified in two ways:
//   - by symbol: from_symbol + to_symbol (e.g. "TON", "USDT")
//   - by jetton minter address: from_jetton_minter + to_jetton_minter (use "ton" for native TON)
type OrderBookRequestHTTP struct {
	FromSymbol       string `form:"from_symbol"`
	ToSymbol         string `form:"to_symbol"`
	FromJettonMinter string `form:"from_jetton_minter"`
	ToJettonMinter   string `form:"to_jetton_minter"`
	Limit            int64  `form:"limit"`
	// MinOrderUsd filters out individual orders whose value is less than this
	// number of USD dollars. Only applied when one side of the pair is a
	// stablecoin (USDT/USDC/USD₮); otherwise ignored. Default: 40. Pass 0 to disable.
	MinOrderUsd *int64 `form:"min_order_usd"`
}

// OrderBookLevel represents a single price level in the order book
type OrderBookLevel struct {
	PriceRate        string `json:"price_rate"`
	TotalAmount      int64  `json:"total_amount"`
	OrderCount       int64  `json:"order_count"`
	TotalValue       string `json:"total_value"`
	CumulativeAmount int64  `json:"cumulative_amount"`
	CumulativeValue  string `json:"cumulative_value"`
}

// OrderBookResponse represents the full order book for a trading pair
type OrderBookResponse struct {
	FromSymbol   string           `json:"from_symbol"`
	ToSymbol     string           `json:"to_symbol"`
	FromDecimals int              `json:"from_decimals"`
	ToDecimals   int              `json:"to_decimals"`
	Spread       *string          `json:"spread"`
	MidPrice     *string          `json:"mid_price"`
	Asks         []OrderBookLevel `json:"asks"`
	Bids         []OrderBookLevel `json:"bids"`
}

// TradingStatsRequestHTTP represents query parameters for the trading stats endpoint.
type TradingStatsRequestHTTP struct {
	FromSymbol       string `form:"from_symbol"`
	ToSymbol         string `form:"to_symbol"`
	FromJettonMinter string `form:"from_jetton_minter"`
	ToJettonMinter   string `form:"to_jetton_minter"`
}

// StatusStats holds count and volume for a single order status.
type StatusStats struct {
	Count  int64 `json:"count"`
	Volume int64 `json:"volume"`
}

// PeriodStats holds trading statistics for a single time period.
type PeriodStats struct {
	Period      string                 `json:"period"`
	TotalOrders int64                  `json:"total_orders"`
	TotalVolume int64                  `json:"total_volume"`
	Open        StatusStats            `json:"open"`
	Filled      StatusStats            `json:"filled"`
	ByStatus    map[string]StatusStats `json:"by_status"`
}

// TradingStatsResponse represents trading statistics for a pair across multiple time periods.
type TradingStatsResponse struct {
	FromSymbol   string        `json:"from_symbol"`
	ToSymbol     string        `json:"to_symbol"`
	FromDecimals int           `json:"from_decimals"`
	ToDecimals   int           `json:"to_decimals"`
	Periods      []PeriodStats `json:"periods"`
}
