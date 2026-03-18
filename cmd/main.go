package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	docs "api/docs"
	"api/internal/config"
	"api/internal/handler"
	"api/internal/logger"
	appmiddleware "api/internal/middleware"
	"api/internal/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title           API
// @version         1.0
// @description     open4dev api

// @host            api.open4dev.xyz
// @BasePath        /api/v1

// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
func main() {
	cfg := config.InitConfig()
	if cfg.Network.Mainnet {
		docs.SwaggerInfo.Host = "api.open4dev.xyz"
		docs.SwaggerInfo.Description = "open4dev api"
		docs.SwaggerInfo.Title = "API"
	} else {
		docs.SwaggerInfo.Host = "stage-api.open4dev.xyz"
		docs.SwaggerInfo.Description = "stage-open4dev api"
		docs.SwaggerInfo.Title = "Stage API"
	}

	logger.InitLogger(cfg)
	slog.Info("Starting API")

	// Initialize service locator with all services and repositories
	svc, err := services.NewServices(cfg)
	if err != nil {
		slog.Error("Failed to initialize services", "error", err)
		panic(err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			slog.Error("Error closing services", "error", err)
		}
	}()

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	// Use custom recovery with detailed logging
	router.Use(appmiddleware.RecoveryLogger(slog.Default()))
	router.Use(appmiddleware.RequestLogger(slog.Default()))

	// CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"https://tradememe.ai", "https://agentmeme.ai"},
		AllowMethods:     []string{"GET", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	api := router.Group("/api/v1")

	// Add database session middleware first - creates a new DB session for each request
	// This must be before APIKeyAuth since it needs DB session
	api.Use(appmiddleware.DatabaseSession(svc.DB))

	// Add API key authentication middleware
	api.Use(appmiddleware.APIKeyAuth(svc.APIKeysRepo))

	// Add rate limiter middleware - limits requests per API key (falls back to 5 RPS by IP for anonymous)
	api.Use(appmiddleware.RateLimiter(svc.Redis, cfg.Api.RPS, 5, time.Second))

	// Register handlers with service locator
	handler.RegisterHandlers(api, svc)

	// swagger handler
	router.GET("/api/v1/swagger/*any", gin.WrapH(httpSwagger.Handler(
		httpSwagger.URL("/api/v1/swagger/doc.json"),
	)))

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		slog.Info("API started", "port", cfg.Api.Port)
		if err := router.Run(":" + cfg.Api.Port); err != nil {
			slog.Error("API stopped unexpectedly", "error", err)
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	slog.Info("Shutting down gracefully...")
}
