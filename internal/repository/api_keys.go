package repository

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"time"

	dbmodels "api/internal/database/models"
	"api/internal/middleware"
)

type APIKeysRepository interface {
	ValidateKey(ctx context.Context, apiKey string) (*dbmodels.APIKey, error)
	UpdateLastUsed(ctx context.Context, keyID int) error
	GetByHash(ctx context.Context, keyHash string) (*dbmodels.APIKey, error)
}

type apiKeysRepository struct {
}

func NewAPIKeysRepository() APIKeysRepository {
	return &apiKeysRepository{}
}

// HashKey creates a SHA-512 hash of the API key
func HashKey(apiKey string) string {
	hash := sha512.Sum512([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

// ValidateKey validates an API key by hashing it and checking against the database
func (r *apiKeysRepository) ValidateKey(ctx context.Context, apiKey string) (*dbmodels.APIKey, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	keyHash := HashKey(apiKey)

	var apiKeyModel dbmodels.APIKey
	now := time.Now()

	// Check if key exists, is active, and not expired
	stmt := session.WithContext(ctx).
		Where("key_hash = ? AND is_active = ?", keyHash, true).
		Where("(expires_at IS NULL OR expires_at > ?)", now).
		First(&apiKeyModel)

	if stmt.Error != nil {
		return nil, stmt.Error
	}

	return &apiKeyModel, nil
}

// UpdateLastUsed updates the last_used_at timestamp for an API key
func (r *apiKeysRepository) UpdateLastUsed(ctx context.Context, keyID int) error {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	stmt := session.WithContext(ctx).
		Model(&dbmodels.APIKey{}).
		Where("id = ?", keyID).
		Updates(map[string]interface{}{
			"last_used_at": now,
			"updated_at":   now,
		})

	return stmt.Error
}

// GetByHash retrieves an API key by its hash
func (r *apiKeysRepository) GetByHash(ctx context.Context, keyHash string) (*dbmodels.APIKey, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var apiKeyModel dbmodels.APIKey
	stmt := session.WithContext(ctx).
		Where("key_hash = ?", keyHash).
		First(&apiKeyModel)

	if stmt.Error != nil {
		return nil, stmt.Error
	}

	return &apiKeyModel, nil
}
