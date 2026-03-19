package schemas

type OrderStatus string

const (
	OrderStatusCreated      OrderStatus = "created"
	OrderStatusDeployed     OrderStatus = "deployed"
	OrderStatusCancelled    OrderStatus = "cancelled"
	OrderStatusCompleted    OrderStatus = "completed"
	OrderStatusFailed       OrderStatus = "failed"
	OrderStatusPendingMatch OrderStatus = "pending_match"
)

type OrderRequestHTTP struct {
	Id uint64 `uri:"id" binding:"required"`
}

type OrderStatsRequestHTTP struct {
	WalletAddress string `form:"wallet_address"`
}

type OrderStatsResponse struct {
	WalletAddress string           `json:"wallet_address"`
	Total         int64            `json:"total"`
	ByStatus      map[string]int64 `json:"by_status"`
	Open          int64            `json:"open"`   // created + deployed + pending_match
	Closed        int64            `json:"closed"` // cancelled + completed + failed + closed
}

type OrderDeployedTotalsRequestHTTP struct {
	WalletAddress string `form:"wallet_address"`
}

type OrderDeployedTotalsItem struct {
	CoinID      int     `json:"coin_id"`
	Symbol      *string `json:"symbol,omitempty"`
	Name        *string `json:"name,omitempty"`
	TotalAmount int64 `json:"total_amount"`
}

type OrderDeployedTotalsResponse struct {
	WalletAddress string                    `json:"wallet_address"`
	Totals        []OrderDeployedTotalsItem `json:"totals"`
}

type OrderListRequestHTTP struct {
	Offset          int64       `form:"offset"`
	Limit           int64       `form:"limit"`
	Sort            string      `form:"sort"`
	Order           string      `form:"order"`
	OwnerRawAddress string      `form:"owner_raw_address"`
	FromCoinID      int64       `form:"from_coin_id"`
	ToCoinID        int64       `form:"to_coin_id"`
	Status          OrderStatus `form:"status"`
	MinAmount       int64       `form:"min_amount"`
	MaxAmount       int64       `form:"max_amount"`
	MinPriceRate    string      `form:"min_price_rate"`
	MaxPriceRate    string      `form:"max_price_rate"`
	MinSlippage     int64       `form:"min_slippage"`
	MaxSlippage     int64       `form:"max_slippage"`
}
