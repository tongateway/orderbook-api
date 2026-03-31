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

// candleIntervals maps interval key to seconds and max lookback.
var candleIntervals = map[string]struct {
	Seconds    int64
	MaxLookback time.Duration
}{
	"1m":  {60, 24 * time.Hour},
	"5m":  {300, 7 * 24 * time.Hour},
	"15m": {900, 14 * 24 * time.Hour},
	"1h":  {3600, 90 * 24 * time.Hour},
	"4h":  {14400, 180 * 24 * time.Hour},
	"1d":  {86400, 365 * 24 * time.Hour},
}

type CandlesHandler struct {
	cache     *cache.CandlesCache
	coinsRepo repository.CoinsRepository
}

func NewCandlesHandler(c *cache.CandlesCache, coinsRepo repository.CoinsRepository) *CandlesHandler {
	return &CandlesHandler{
		cache:     c,
		coinsRepo: coinsRepo,
	}
}

// @Summary      Get OHLCV candles for a trading pair
// @Description  Builds OHLCV candles from completed orders. Intervals: 1m, 5m, 15m, 1h, 4h, 1d.
// @Description  Pair can be specified by symbols or jetton minter addresses.
// @Tags         orders
// @Accept       json
// @Produce      json
// @Param        from_symbol        query  string  false  "From coin symbol (e.g. TON, AGNT)"
// @Param        to_symbol          query  string  false  "To coin symbol"
// @Param        from_jetton_minter query  string  false  "From jetton minter address"
// @Param        to_jetton_minter   query  string  false  "To jetton minter address"
// @Param        interval           query  string  true   "Candle interval: 1m, 5m, 15m, 1h, 4h, 1d"
// @Param        limit              query  int     false  "Max candles (default 500, max 1000)"
// @Success      200  {object}  schemas.CandlesResponse
// @Failure      400  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /orders/candles [get]
func (h *CandlesHandler) GetCandles(c *gin.Context) {
	var req schemas.CandlesRequestHTTP
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Set("error", err)
		slog.WarnContext(c.Request.Context(), "invalid candles query parameters", "error", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate interval.
	ivInfo, ok := candleIntervals[req.Interval]
	if !ok {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "invalid interval, use: 1m, 5m, 15m, 1h, 4h, 1d",
		})
		return
	}

	// Resolve pair.
	hasSymbols := req.FromSymbol != "" && req.ToSymbol != ""
	hasMinters := req.FromJettonMinter != "" && req.ToJettonMinter != ""

	if !hasSymbols && !hasMinters {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "specify pair using from_symbol+to_symbol or from_jetton_minter+to_jetton_minter",
		})
		return
	}
	if hasSymbols && hasMinters {
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
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		toCoin, err = resolveCoinBySymbol(ctx, h.coinsRepo, req.ToSymbol)
		if err != nil {
			c.Set("error", err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		fromCoin, err = resolveCoinByMinter(ctx, h.coinsRepo, req.FromJettonMinter)
		if err != nil {
			c.Set("error", err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		toCoin, err = resolveCoinByMinter(ctx, h.coinsRepo, req.ToJettonMinter)
		if err != nil {
			c.Set("error", err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	if fromCoin.ID == toCoin.ID {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "from and to coins must be different"})
		return
	}

	limit := req.Limit
	if limit <= 0 || limit > 1000 {
		limit = 500
	}

	since := time.Now().Add(-ivInfo.MaxLookback)

	rows, err := h.cache.Get(ctx, fromCoin.ID, toCoin.ID, ivInfo.Seconds, since, limit)
	if err != nil {
		c.Set("error", err)
		fullErr := middleware.FormatErrorFull(err)
		slog.ErrorContext(ctx, "failed to get candles",
			"error", err,
			"error_full", fullErr,
			"interval", req.Interval,
			"from_coin_id", fromCoin.ID,
			"to_coin_id", toCoin.ID,
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	candles := make([]schemas.CandleItem, len(rows))
	for i, r := range rows {
		candles[i] = schemas.CandleItem{
			Timestamp: r.BucketTs,
			Open:      r.Open,
			High:      r.High,
			Low:       r.Low,
			Close:     r.Close,
			Volume:    r.Volume,
		}
	}

	c.JSON(http.StatusOK, schemas.CandlesResponse{
		FromSymbol:   fromCoin.Symbol,
		ToSymbol:     toCoin.Symbol,
		FromDecimals: fromCoin.Decimals,
		ToDecimals:   toCoin.Decimals,
		Interval:     req.Interval,
		Candles:      candles,
	})
}
