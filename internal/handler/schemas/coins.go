package schemas

type CoinRequestHTTP struct {
	Id uint64 `uri:"id" binding:"required"`
}

type CoinListRequestHTTP struct {
	Offset int64  `form:"offset"`
	Limit  int64  `form:"limit"`
	Sort   string `form:"sort"`
	Order  string `form:"order"`
}

// CoinPriceRequestHTTP represents query parameters for the coin price endpoint.
// Coin can be identified by name, symbol, or jetton minter address.
type CoinPriceRequestHTTP struct {
	Name         string `form:"name"`
	Symbol       string `form:"symbol"`
	JettonMinter string `form:"jetton_minter"`
}

// CoinPricePair represents price data for one trading pair of the requested coin.
type CoinPricePair struct {
	CounterCoinID       int    `json:"counter_coin_id"`
	CounterCoinSymbol   string `json:"counter_coin_symbol"`
	CounterCoinDecimals int    `json:"counter_coin_decimals"`
	BestAsk             *int64 `json:"best_ask"`
	BestBid             *int64 `json:"best_bid"`
	MidPrice            *int64 `json:"mid_price"`
	Spread              *int64 `json:"spread"`
	AskTotalAmount      int64  `json:"ask_total_amount"`
	BidTotalAmount      int64  `json:"bid_total_amount"`
	AskOrderCount       int64  `json:"ask_order_count"`
	BidOrderCount       int64  `json:"bid_order_count"`
}

// CoinPriceCoinInfo holds basic coin identification for the response.
type CoinPriceCoinInfo struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

// CoinPriceResponse represents the full price response for a coin.
type CoinPriceResponse struct {
	Coin  CoinPriceCoinInfo `json:"coin"`
	Pairs []CoinPricePair   `json:"pairs"`
}
