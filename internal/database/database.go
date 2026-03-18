package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"api/internal/config"
	"api/internal/middleware"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// buildPostgresDSN builds PostgreSQL DSN from configuration
func buildPostgresDSN(cfg *config.DatabaseConfig) string {
	// If DSN is explicitly provided, use it
	if cfg.DSN != "" {
		return cfg.DSN
	}

	// Build DSN from individual parameters
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}

	port := cfg.Port
	if port == "" {
		port = "5432"
	}

	user := cfg.User
	if user == "" {
		user = "postgres"
	}

	password := cfg.Password
	dbname := cfg.DBName
	if dbname == "" {
		dbname = "postgres"
	}

	sslmode := cfg.SSLMode
	if sslmode == "" {
		sslmode = "disable" // Default to disable for local development
	}

	// Format: postgres://user:password@host:port/dbname?sslmode=disable
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	return dsn
}

// NewDatabase creates a new database connection based on configuration
func NewDatabase(cfg *config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	dsn := buildPostgresDSN(&cfg.Database)
	dialector = postgres.Open(dsn)

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: middleware.NewGormLogger(slog.Default(), 200*time.Millisecond, "trace"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	postgresDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Set connection pool settings
	postgresDB.SetMaxIdleConns(10)
	postgresDB.SetMaxOpenConns(100)

	slog.Info("Database connection established", "type", cfg.Database.Type, "host", cfg.Database.Host)

	return db, nil
}

func GetDBSessionFromContext(ctx context.Context) (*gorm.DB, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// CloseDatabase closes the database connection
func CloseDatabase(db *gorm.DB) error {
	postgresDB, err := db.DB()
	if err != nil {
		return err
	}
	return postgresDB.Close()
}
