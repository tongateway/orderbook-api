package models

import "time"

type Wallet struct {
	ID         int       `gorm:"column:id;type:integer;primaryKey;uniqueIndex:wallets_pkey"`
	UserID     *int      `gorm:"column:user_id;type:integer"`
	RawAddress *string   `gorm:"column:raw_address;type:character varying(67);uniqueIndex:unique_raw_address;not null"`
	CreatedAt  time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()"`

	// Relations
	User *User `gorm:"foreignKey:UserID;references:ID"`
}

func (Wallet) TableName() string {
	return "wallets"
}
