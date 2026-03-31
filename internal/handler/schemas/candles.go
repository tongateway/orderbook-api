package schemas

// CandlesRequestHTTP represents query parameters for the candles endpoint.
type CandlesRequestHTTP struct {
	FromSymbol       string `form:"from_symbol"`
	ToSymbol         string `form:"to_symbol"`
	FromJettonMinter string `form:"from_jetton_minter"`
	ToJettonMinter   string `form:"to_jetton_minter"`
	Interval         string `form:"interval" binding:"required"`
	Limit            int    `form:"limit"`
}

// CandleItem represents a single OHLCV candle.
type CandleItem struct {
	Timestamp int64  `json:"t"`
	Open      string `json:"o"`
	High      string `json:"h"`
	Low       string `json:"l"`
	Close     string `json:"c"`
	Volume    int64  `json:"v"`
}

// CandlesResponse represents the candles endpoint response.
type CandlesResponse struct {
	FromSymbol   string       `json:"from_symbol"`
	ToSymbol     string       `json:"to_symbol"`
	FromDecimals int          `json:"from_decimals"`
	ToDecimals   int          `json:"to_decimals"`
	Interval     string       `json:"interval"`
	Candles      []CandleItem `json:"candles"`
}
