package handler

import (
	"api/internal/services"

	"github.com/gin-gonic/gin"
)

// RegisterHandlers registers all HTTP handlers with the router
// Repositories are provided through the service locator pattern
func RegisterHandlers(router *gin.RouterGroup, svc *services.Services) {
	// Initialize handlers with repositories from service locator
	coinsHandler := NewCoinsHandler(svc.CoinsRepo)
	coinPriceHandler := NewCoinPriceHandler(svc.CoinPriceCache, svc.CoinsRepo)
	orderHandler := NewOrderHandler(svc.OrderRepo)
	orderBookHandler := NewOrderBookHandler(svc.OrderBookCache, svc.CoinsRepo)
	tradingStatsHandler := NewTradingStatsHandler(svc.TradingStatsCache, svc.CoinsRepo)
	candlesHandler := NewCandlesHandler(svc.CandlesCache, svc.CoinsRepo)
	agentLeaderboardHandler := NewAgentLeaderboardHandler(svc.AgentLeaderboardCache, svc.CoinsRepo)
	batchContextHandler := NewBatchContextHandler(svc.BatchContextCache)
	vaultHandler := NewVaultHandler(svc.VaultRepo)

	// Register routes
	router.GET("/coins", coinsHandler.List)
	router.GET("/coins/price", coinPriceHandler.GetPrice)
	router.GET("/coins/:id", coinsHandler.GetByID)
	router.GET("/orders", orderHandler.List)
	router.GET("/orders/book", orderBookHandler.GetOrderBook)
	router.GET("/orders/stats", orderHandler.Stats)
	router.GET("/orders/deployed-totals", orderHandler.DeployedTotals)
	router.GET("/orders/trading-stats", tradingStatsHandler.GetTradingStats)
	router.GET("/orders/candles", candlesHandler.GetCandles)
	router.GET("/orders/agent-leaderboard", agentLeaderboardHandler.GetAgentLeaderboard)
	router.POST("/orders/batch-context", batchContextHandler.BatchContext)
	router.GET("/orders/:id", orderHandler.GetByID)
	router.GET("/vaults", vaultHandler.List)
	router.GET("/vaults/:id", vaultHandler.GetByID)
}
