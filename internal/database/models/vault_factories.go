package models

import "time"

type VaultFactory struct {
	ID                      int       `gorm:"column:id;type:integer;primaryKey;uniqueIndex:vault_factories_pkey"`
	Version                 float64   `gorm:"column:version;type:double precision;not null"`
	RawAddress              string    `gorm:"column:raw_address;type:character varying(67);uniqueIndex:vault_factories_raw_address_key;not null"`
	OwnerAddress            string    `gorm:"column:owner_address;type:character varying(67);not null"`
	CreatedAt               time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
	VaultCode               string    `gorm:"column:vault_code;type:character varying;not null"`
	OrderCode               string    `gorm:"column:order_code;type:character varying;not null"`
	MatcherFeeCollectorCode string    `gorm:"column:matcher_fee_collector_code;type:character varying;not null"`
	Type                    string    `gorm:"column:type;type:character varying" json:"type"`
}

func (VaultFactory) TableName() string {
	return "vault_factories"
}
