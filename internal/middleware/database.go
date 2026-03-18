package middleware

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type contextKey string

const (
	dbSessionKey                   = "db_session" // For Gin context
	dbSessionContextKey contextKey = "db_session" // For standard context
)

// DatabaseSession creates a new database session for each request
// This ensures that each request has its own isolated database session
func DatabaseSession(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create a new session from the connection pool for this request
		session := db.Session(&gorm.Session{})

		// Store the session in both Gin context and request context
		c.Set(dbSessionKey, session)
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), dbSessionContextKey, session))

		// Continue with the request
		c.Next()

		// Session will be automatically cleaned up when the request completes
	}
}

// GetDBSession retrieves the database session from the Gin context
func GetDBSession(c *gin.Context) *gorm.DB {
	if session, exists := c.Get(dbSessionKey); exists {
		if db, ok := session.(*gorm.DB); ok {
			return db
		}
	}
	return nil
}

// GetDBSessionFromContext retrieves the database session from the standard context
func GetDBSessionFromContext(ctx context.Context) (*gorm.DB, error) {
	if session := ctx.Value(dbSessionContextKey); session != nil {
		if db, ok := session.(*gorm.DB); ok {
			return db, nil
		}
	}
	return nil, fmt.Errorf("database session not found")
}
