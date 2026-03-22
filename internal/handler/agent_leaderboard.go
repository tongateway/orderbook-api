package handler

import (
	"log/slog"
	"net/http"

	"api/internal/cache"
	"api/internal/handler/schemas"
	"api/internal/middleware"
	"api/internal/repository"

	"github.com/gin-gonic/gin"
)

type AgentLeaderboardHandler struct {
	cache     *cache.AgentLeaderboardCache
	coinsRepo repository.CoinsRepository
}

func NewAgentLeaderboardHandler(c *cache.AgentLeaderboardCache, coinsRepo repository.CoinsRepository) *AgentLeaderboardHandler {
	return &AgentLeaderboardHandler{
		cache:     c,
		coinsRepo: coinsRepo,
	}
}

// @Summary      Get agent leaderboard for a coin
// @Description  Returns aggregated trading stats per agent (wallet) for orders involving the given coin, sorted by completed volume
// @Tags         orders
// @Accept       json
// @Produce      json
// @Param        coin_symbol    query  string  false  "Coin symbol (e.g. AGNT, TON, NOT)"
// @Param        jetton_minter  query  string  false  "Jetton minter raw address (use 'ton' for TON)"
// @Param        limit          query  int     false  "Limit (default 50, max 200)"
// @Param        offset         query  int     false  "Offset (default 0)"
// @Success      200  {object}  schemas.AgentLeaderboardResponse
// @Failure      400  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /orders/agent-leaderboard [get]
func (h *AgentLeaderboardHandler) GetAgentLeaderboard(c *gin.Context) {
	var req schemas.AgentLeaderboardRequestHTTP
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Set("error", err)
		slog.WarnContext(c.Request.Context(), "invalid query parameters", "error", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Resolve coin
	var coin coinInfo
	var err error
	if req.CoinSymbol != "" {
		coin, err = resolveCoinBySymbol(c.Request.Context(), h.coinsRepo, req.CoinSymbol)
	} else if req.JettonMinter != "" {
		coin, err = resolveCoinByMinter(c.Request.Context(), h.coinsRepo, req.JettonMinter)
	} else {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "coin_symbol or jetton_minter is required"})
		return
	}
	if err != nil {
		c.Set("error", err)
		fullErr := middleware.FormatErrorFull(err)
		slog.WarnContext(c.Request.Context(), "failed to resolve coin", "error", err, "error_full", fullErr)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rows, err := h.cache.Get(c.Request.Context(), coin.ID)
	if err != nil {
		c.Set("error", err)
		fullErr := middleware.FormatErrorFull(err)
		slog.ErrorContext(c.Request.Context(), "failed to get agent leaderboard", "error", err, "error_full", fullErr)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Defaults
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	// Paginate
	total := len(rows)
	if offset >= total {
		rows = nil
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		rows = rows[offset:end]
	}

	// Build response with ranks
	agents := make([]schemas.AgentLeaderboardEntry, 0, len(rows))
	for i, r := range rows {
		agents = append(agents, schemas.AgentLeaderboardEntry{
			Rank:            offset + i + 1,
			RawAddress:      r.RawAddress,
			TotalOrders:     r.TotalOrders,
			CompletedOrders: r.CompletedOrders,
			DeployedOrders:  r.DeployedOrders,
			CompletedVolume: r.CompletedVolume,
			BuyVolume:       r.BuyVolume,
			SellVolume:      r.SellVolume,
		})
	}

	c.JSON(http.StatusOK, schemas.AgentLeaderboardResponse{
		CoinID:   coin.ID,
		Symbol:   coin.Symbol,
		Decimals: coin.Decimals,
		Agents:   agents,
	})
}
