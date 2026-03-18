package models

import "time"

type Coin struct {
	ID                  int       `gorm:"column:id;type:integer;primaryKey;uniqueIndex:coins_pkey" json:"id"`
	IDCoingecko         *string   `gorm:"column:id_coingecko;type:character varying;uniqueIndex:coins_id_coingecko_key" json:"-"`
	Name                *string   `gorm:"column:name;type:character varying" json:"name"`
	Symbol              *string   `gorm:"column:symbol;type:character varying" json:"symbol"`
	CreatedAt           time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()" json:"created_at"`
	TonRawAddress       string    `gorm:"column:ton_raw_address;type:character varying;uniqueIndex:coins_ton_raw_address_key;not null" json:"ton_raw_address"`
	HexJettonWalletCode *string   `gorm:"column:hex_jetton_wallet_code;type:character varying" json:"hex_jetton_wallet_code"`
	JettonContent       *string   `gorm:"column:jetton_content;type:character varying" json:"jetton_content"`
	Decimals            *int      `gorm:"column:decimals;type:integer" json:"decimals"`
}

func (Coin) TableName() string {
	return "coins"
}
