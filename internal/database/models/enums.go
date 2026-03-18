package models

// PostgreSQL enum types represented as Go string types

type OrderStatus string

const (
	OrderStatusCreated      OrderStatus = "created"
	OrderStatusDeployed     OrderStatus = "deployed"
	OrderStatusCancelled    OrderStatus = "cancelled"
	OrderStatusCompleted    OrderStatus = "completed"
	OrderStatusFailed       OrderStatus = "failed"
	OrderStatusPendingMatch OrderStatus = "pending_match"
	OrderStatusClosed       OrderStatus = "closed"
)

type OrderType string

const (
	OrderTypeJettonToJetton OrderType = "jetton_to_jetton"
	OrderTypeJettonToTon    OrderType = "jetton_to_ton"
	OrderTypeTonToJetton    OrderType = "ton_to_jetton"
)

type TransactionAction string

const (
	TransactionActionCreateOrder   TransactionAction = "create_order"
	TransactionActionCancelOrder   TransactionAction = "cancel_order"
	TransactionActionCompleteOrder TransactionAction = "complete_order"
	TransactionActionFailOrder     TransactionAction = "fail_order"
)

type TransactionStatus string

const (
	TransactionStatusCreated   TransactionStatus = "created"
	TransactionStatusPending   TransactionStatus = "pending"
	TransactionStatusCompleted TransactionStatus = "completed"
	TransactionStatusFailed    TransactionStatus = "failed"
)

type VaultType string

const (
	VaultTypeJetton VaultType = "jetton"
	VaultTypeTon    VaultType = "ton"
)
