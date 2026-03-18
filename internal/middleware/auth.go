package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	dbmodels "api/internal/database/models"

	"log/slog"

	"github.com/gin-gonic/gin"
)

// APIKeysValidator is an interface for validating API keys
// This interface is defined here to avoid circular imports
type APIKeysValidator interface {
	ValidateKey(ctx context.Context, apiKey string) (*dbmodels.APIKey, error)
	UpdateLastUsed(ctx context.Context, keyID int) error
}

// APIKeyAuth creates a middleware that validates API keys from the Authorization header.
// Expected format: "Authorization: Bearer <api_key>" or "Authorization: <api_key>"
// All endpoints are public: if no token is provided the request is allowed anonymously
// (rate-limited to 1 RPS by IP). If a token is provided but invalid, 401 is returned.
func APIKeyAuth(apiKeysValidator APIKeysValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get API key from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// No token — anonymous request, allow through (rate limiter will enforce 1 RPS by IP)
			c.Next()
			return
		}

		// Extract API key from header
		// Support both "Bearer <key>" and "<key>" formats
		apiKey := strings.TrimSpace(authHeader)
		lowerHeader := strings.ToLower(apiKey)
		if strings.HasPrefix(lowerHeader, "bearer ") {
			// Remove "Bearer " prefix (case-insensitive)
			apiKey = strings.TrimSpace(apiKey[7:]) // "bearer " is 7 characters
		}

		if apiKey == "" {
			slog.WarnContext(c.Request.Context(), "Empty API key",
				"ip", c.ClientIP(),
				"path", c.Request.URL.Path,
			)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Invalid API key",
			})
			c.Abort()
			return
		}

		// Validate API key
		keyHash := hashKey(apiKey)
		ctx := c.Request.Context()

		// Validate API key against database
		apiKeyModel, err := apiKeysValidator.ValidateKey(ctx, apiKey)
		if err != nil {
			slog.WarnContext(ctx, "Invalid API key",
				"ip", c.ClientIP(),
				"path", c.Request.URL.Path,
				"key_prefix", getKeyPrefix(apiKey),
				"error", err,
			)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Invalid API key",
			})
			c.Abort()
			return
		}

		// Update last_used_at (non-blocking to avoid slowing down request)
		go func() {
			if err := apiKeysValidator.UpdateLastUsed(context.Background(), apiKeyModel.ID); err != nil {
				slog.WarnContext(context.Background(), "Failed to update API key last_used_at",
					"key_id", apiKeyModel.ID,
					"error", err,
				)
			}
		}()

		// Store API key info in context
		c.Set("api_key_hash", keyHash)
		c.Set("api_key_id", apiKeyModel.ID)
		c.Set("api_key_rate_limit", apiKeyModel.RateLimit)
		if apiKeyModel.UserID != nil {
			c.Set("user_id", *apiKeyModel.UserID)
		}

		c.Next()
	}
}

// getKeyPrefix returns first 8 characters of the key for logging (for security)
func getKeyPrefix(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "****"
}

// hashKey creates a SHA-256 hash of the API key
func hashKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}
