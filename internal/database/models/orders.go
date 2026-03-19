package models

import "time"

type Order struct {
	ID                 int         `gorm:"column:id;type:integer;primaryKey;uniqueIndex:orders_pkey" json:"id"`
	Title              string      `gorm:"column:title;type:character varying(200);not null" json:"title"`
	UserID             *int        `gorm:"column:user_id;type:integer" json:"user_id"`
	RawAddress         *string     `gorm:"column:raw_address;type:character varying(67);uniqueIndex:orders_raw_address_key" json:"raw_address"`
	WalletID           *int        `gorm:"column:wallet_id;type:integer;not null" json:"wallet_id"`
	CreatedAt          time.Time   `gorm:"column:created_at;type:timestamptz;not null;default:now()" json:"created_at"`
	VaultID            *int        `gorm:"column:vault_id;type:integer;not null" json:"vault_id"`
	Status             OrderStatus `gorm:"column:status;type:orderstatus;not null;default:'created'" json:"status"`
	Amount             *int64      `gorm:"column:amount;type:bigint" json:"amount"`
	InitialAmount      *int64      `gorm:"column:initial_amount;type:bigint" json:"initial_amount"`
	PriceRate          *string     `gorm:"column:price_rate;type:numeric" json:"price_rate"`
	Slippage           *int64      `gorm:"column:slippage;type:bigint" json:"slippage"`
	FromCoinID         int         `gorm:"column:from_coin_id;type:integer" json:"from_coin_id"`
	ToCoinID           int         `gorm:"column:to_coin_id;type:integer" json:"to_coin_id"`
	ProviderRawAddress *string     `gorm:"column:provider_raw_address;type:character varying(67)" json:"provider_raw_address"`
	FeeNum             *int        `gorm:"column:fee_num;type:integer" json:"fee_num"`
	FeeDenom           *int        `gorm:"column:fee_denom;type:integer" json:"fee_denom"`
	MatcherFeeNum      *int        `gorm:"column:matcher_fee_num;type:integer" json:"matcher_fee_num"`
	MatcherFeeDenom    *int        `gorm:"column:matcher_fee_denom;type:integer" json:"matcher_fee_denom"`
	OppositeVaultID    *int        `gorm:"column:opposite_vault_id;type:integer" json:"opposite_vault_id"`
	PendingMatchAt     *time.Time  `gorm:"column:pending_match_at;type:timestamptz" json:"pending_match_at"`

	// Relations
	User          *User   `gorm:"foreignKey:UserID;references:ID" json:"-"`
	Vault         *Vault  `gorm:"foreignKey:VaultID;references:ID" json:"-"`
	OppositeVault *Vault  `gorm:"foreignKey:OppositeVaultID;references:ID" json:"-"`
	Wallet        *Wallet `gorm:"foreignKey:WalletID;references:ID" json:"-"`
	FromCoin      *Coin   `gorm:"foreignKey:FromCoinID;references:ID" json:"-"`
	ToCoin        *Coin   `gorm:"foreignKey:ToCoinID;references:ID" json:"-"`
}

func (Order) TableName() string {
	return "orders"
}
