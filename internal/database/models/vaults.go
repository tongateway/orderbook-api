package models

import "time"

type Vault struct {
	ID                  int       `gorm:"column:id;type:integer;primaryKey;uniqueIndex:vaults_pkey"`
	FactoryID           int       `gorm:"column:factory_id;type:integer;not null"`
	RawAddress          *string   `gorm:"column:raw_address;type:character varying(67);uniqueIndex:vaults_raw_address_key"`
	CreatedAt           time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
	JettonMinterAddress *string   `gorm:"column:jetton_minter_address;type:character varying(67)"`
	Type                VaultType `gorm:"column:_type;type:vaulttype;not null"`
	JettonWalletCode    *string   `gorm:"column:jetton_wallet_code;type:character varying"`

	// Relations
	Factory *VaultFactory `gorm:"foreignKey:FactoryID;references:ID"`
}

func (Vault) TableName() string {
	return "vaults"
}
