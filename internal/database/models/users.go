package models

import "time"

type User struct {
	ID         int       `gorm:"column:id;type:integer;primaryKey;uniqueIndex:users_pkey"`
	TelegramID int       `gorm:"column:telegram_id;type:integer;uniqueIndex:users_telegram_id_key;not null"`
	Username   string    `gorm:"column:username;type:character varying;not null"`
	FirstName  *string   `gorm:"column:first_name;type:character varying"`
	LastName   *string   `gorm:"column:last_name;type:character varying"`
	CreatedAt  time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
}

func (User) TableName() string {
	return "users"
}
