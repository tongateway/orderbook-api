package models

import "time"

type APIKey struct {
	ID          int        `gorm:"column:id;type:integer;primaryKey;uniqueIndex:api_keys_pkey"`
	KeyHash     string     `gorm:"column:key_hash;type:character varying(255);uniqueIndex:api_keys_key_hash_key;not null"`
	Name        *string    `gorm:"column:name;type:character varying(255)"`
	Description *string    `gorm:"column:description;type:text"`
	UserID      *int       `gorm:"column:user_id;type:integer;index:idx_api_keys_user_id"`
	IsActive    bool       `gorm:"column:is_active;type:boolean;not null;default:true;index:idx_api_keys_active"`
	RateLimit   int        `gorm:"column:rate_limit;type:integer;not null;default:1"`
	LastUsedAt  *time.Time `gorm:"column:last_used_at;type:timestamptz"`
	ExpiresAt   *time.Time `gorm:"column:expires_at;type:timestamptz;index:idx_api_keys_expires"`
	CreatedAt   time.Time  `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;type:timestamptz;not null;default:now()"`

	// Relations
	User *User `gorm:"foreignKey:UserID;references:ID"`
}

func (APIKey) TableName() string {
	return "api_keys"
}
