package schemas

type AgentLeaderboardRequestHTTP struct {
	CoinSymbol    string `form:"coin_symbol"`
	JettonMinter  string `form:"jetton_minter"`
	Limit         int    `form:"limit"`
	Offset        int    `form:"offset"`
}

type AgentLeaderboardEntry struct {
	Rank            int    `json:"rank"`
	RawAddress      string `json:"raw_address"`
	TotalOrders     int64  `json:"total_orders"`
	CompletedOrders int64  `json:"completed_orders"`
	DeployedOrders  int64  `json:"deployed_orders"`
	CompletedVolume int64  `json:"completed_volume"`
	BuyVolume       int64  `json:"buy_volume"`
	SellVolume      int64  `json:"sell_volume"`
}

type AgentLeaderboardResponse struct {
	CoinID   int                     `json:"coin_id"`
	Symbol   string                  `json:"symbol"`
	Decimals int                     `json:"decimals"`
	Agents   []AgentLeaderboardEntry `json:"agents"`
}
