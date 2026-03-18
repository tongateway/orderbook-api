package handler

import (
	"log/slog"
	"net/http"
	"time"

	"api/internal/cache"
	"api/internal/handler/schemas"
	"api/internal/middleware"
	"api/internal/repository"

	"github.com/gin-gonic/gin"
)

// tradingStatsPeriods defines the time periods for trading stats.
var tradingStatsPeriods = []struct {
	Label    string
	Duration time.Duration
}{
	{"1h", 1 * time.Hour},
	{"24h", 24 * time.Hour},
	{"7d", 7 * 24 * time.Hour},
	{"30d", 30 * 24 * time.Hour},
}

type TradingStatsHandler struct {
	cache     *cache.TradingStatsCache
	coinsRepo repository.CoinsRepository
}

func NewTradingStatsHandler(c *cache.TradingStatsCache, coinsRepo repository.CoinsRepository) *TradingStatsHandler {
	return &TradingStatsHandler{
		cache:     c,
		coinsRepo: coinsRepo,
	}
}

// @Summary      Get trading statistics for a pair
// @Description  Returns order counts and volumes grouped by status for 1h, 24h, 7d, 30d periods.
// @Description  Pair can be specified by symbols or jetton minter addresses (same as order book).
// @Tags         orders
// @Accept       json
// @Produce      json
// @Param        from_symbol        query  string  false  "From coin symbol (e.g. TON, USDT)"
// @Param        to_symbol          query  string  false  "To coin symbol"
// @Param        from_jetton_minter query  string  false  "From jetton minter address"
// @Param        to_jetton_minter   query  string  false  "To jetton minter address"
// @Success      200  {object}  schemas.TradingStatsResponse
// @Failure      400  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /orders/trading-stats [get]
func (h *TradingStatsHandler) GetTradingStats(c *gin.Context) {
	var req schemas.TradingStatsRequestHTTP
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Set("error", err)
		slog.WarnContext(c.Request.Context(), "invalid trading stats query parameters", "error", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hasSymbols := req.FromSymbol != "" && req.ToSymbol != ""
	hasMinters := req.FromJettonMinter != "" && req.ToJettonMinter != ""

	if !hasSymbols && !hasMinters {
		c.Set("error", "pair not specified")
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "specify pair using from_symbol+to_symbol or from_jetton_minter+to_jetton_minter",
		})
		return
	}

	if hasSymbols && hasMinters {
		c.Set("error", "ambiguous pair specification")
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "use either from_symbol+to_symbol or from_jetton_minter+to_jetton_minter, not both",
		})
		return
	}

	ctx := c.Request.Context()

	var fromCoin, toCoin coinInfo
	var err error

	if hasSymbols {
		fromCoin, err = resolveCoinBySymbol(ctx, h.coinsRepo, req.FromSymbol)
		if err != nil {
			c.Set("error", err)
			slog.WarnContext(ctx, "failed to resolve from_symbol", "symbol", req.FromSymbol, "error", err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		toCoin, err = resolveCoinBySymbol(ctx, h.coinsRepo, req.ToSymbol)
		if err != nil {
			c.Set("error", err)
			slog.WarnContext(ctx, "failed to resolve to_symbol", "symbol", req.ToSymbol, "error", err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		fromCoin, err = resolveCoinByMinter(ctx, h.coinsRepo, req.FromJettonMinter)
		if err != nil {
			c.Set("error", err)
			slog.WarnContext(ctx, "failed to resolve from_jetton_minter", "minter", req.FromJettonMinter, "error", err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		toCoin, err = resolveCoinByMinter(ctx, h.coinsRepo, req.ToJettonMinter)
		if err != nil {
			c.Set("error", err)
			slog.WarnContext(ctx, "failed to resolve to_jetton_minter", "minter", req.ToJettonMinter, "error", err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	if fromCoin.ID == toCoin.ID {
		c.Set("error", "from and to coins must be different")
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "from and to coins must be different"})
		return
	}

	now := time.Now()

	periods := make([]schemas.PeriodStats, 0, len(tradingStatsPeriods))
	for _, p := range tradingStatsPeriods {
		since := now.Add(-p.Duration)
		rows, err := h.cache.Get(ctx, fromCoin.ID, toCoin.ID, p.Label, since)
		if err != nil {
			c.Set("error", err)
			fullErr := middleware.FormatErrorFull(err)
			slog.ErrorContext(ctx, "failed to get trading stats",
				"error", err,
				"error_full", fullErr,
				"period", p.Label,
				"from_coin_id", fromCoin.ID,
				"to_coin_id", toCoin.ID,
			)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		byStatus := make(map[string]schemas.StatusStats)
		var totalOrders int64
		var totalVolume int64
		for _, r := range rows {
			byStatus[r.Status] = schemas.StatusStats{
				Count:  r.Count,
				Volume: r.Volume,
			}
			totalOrders += r.Count
			totalVolume += r.Volume
		}

		periods = append(periods, schemas.PeriodStats{
			Period:      p.Label,
			TotalOrders: totalOrders,
			TotalVolume: totalVolume,
			ByStatus:    byStatus,
		})
	}

	c.JSON(http.StatusOK, schemas.TradingStatsResponse{
		FromSymbol:   fromCoin.Symbol,
		ToSymbol:     toCoin.Symbol,
		FromDecimals: fromCoin.Decimals,
		ToDecimals:   toCoin.Decimals,
		Periods:      periods,
	})
}
