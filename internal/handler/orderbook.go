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

type OrderBookHandler struct {
	cache     *cache.OrderBookCache
	coinsRepo repository.CoinsRepository
}

func NewOrderBookHandler(c *cache.OrderBookCache, coinsRepo repository.CoinsRepository) *OrderBookHandler {
	return &OrderBookHandler{
		cache:     c,
		coinsRepo: coinsRepo,
	}
}

// @Summary      Get order book for a trading pair
// @Description  Returns aggregated price levels (asks and bids) for deployed orders of a given pair.
// @Description  Pair can be specified by symbols (from_symbol + to_symbol) or by jetton minter addresses (from_jetton_minter + to_jetton_minter). Use "ton" for native TON.
// @Description  Asks = orders selling from_coin for to_coin, sorted by price_rate ASC (best ask first).
// @Description  Bids = orders selling to_coin for from_coin, sorted by price_rate DESC (best bid first).
// @Description  Amounts are returned in human-readable format (adjusted for token decimals).
// @Tags         orders
// @Accept       json
// @Produce      json
// @Param        from_symbol        query  string  false  "From coin symbol (e.g. TON, USDT)"
// @Param        to_symbol          query  string  false  "To coin symbol"
// @Param        from_jetton_minter query  string  false  "From jetton minter address (use 'ton' for native TON)"
// @Param        to_jetton_minter   query  string  false  "To jetton minter address (use 'ton' for native TON)"
// @Param        limit              query  int     false  "Max number of price levels per side (default 100, max 500)"
// @Success      200  {object}  schemas.OrderBookResponse
// @Failure      400  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /orders/book [get]
func (h *OrderBookHandler) GetOrderBook(c *gin.Context) {
	var req schemas.OrderBookRequestHTTP
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Set("error", err)
		fullErr := middleware.FormatErrorFull(err)
		slog.WarnContext(c.Request.Context(), "invalid order book query parameters",
			"error", err,
			"error_full", fullErr,
			"query", c.Request.URL.RawQuery,
		)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Determine which identification mode is used
	hasSymbols := req.FromSymbol != "" && req.ToSymbol != ""
	hasMinters := req.FromJettonMinter != "" && req.ToJettonMinter != ""

	if !hasSymbols && !hasMinters {
		c.Set("error", "pair not specified")
		slog.WarnContext(c.Request.Context(), "pair not specified", "query", c.Request.URL.RawQuery)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "specify pair using from_symbol+to_symbol or from_jetton_minter+to_jetton_minter",
		})
		return
	}

	if hasSymbols && hasMinters {
		c.Set("error", "ambiguous pair specification")
		slog.WarnContext(c.Request.Context(), "both symbol and minter params provided")
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "use either from_symbol+to_symbol or from_jetton_minter+to_jetton_minter, not both",
		})
		return
	}

	var fromCoin, toCoin coinInfo
	var err error

	ctx := c.Request.Context()

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
		slog.WarnContext(ctx, "from and to coins are the same",
			"from_coin_id", fromCoin.ID,
			"to_coin_id", toCoin.ID,
		)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "from and to coins must be different"})
		return
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 10
	}
	if limit > 25 {
		limit = 25
	}

	// Asks: orders selling from_coin for to_coin, sorted by price ASC (best ask = lowest)
	asks, err := h.cache.Get(ctx, fromCoin.ID, toCoin.ID)
	if err != nil {
		c.Set("error", err)
		fullErr := middleware.FormatErrorFull(err)
		slog.ErrorContext(ctx, "failed to get order book asks",
			"error", err,
			"error_full", fullErr,
			"from_coin_id", fromCoin.ID,
			"to_coin_id", toCoin.ID,
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Bids: orders selling to_coin for from_coin, sorted by price DESC (best bid = highest)
	bids, err := h.cache.Get(ctx, toCoin.ID, fromCoin.ID)
	if err != nil {
		c.Set("error", err)
		fullErr := middleware.FormatErrorFull(err)
		slog.ErrorContext(ctx, "failed to get order book bids",
			"error", err,
			"error_full", fullErr,
			"from_coin_id", toCoin.ID,
			"to_coin_id", fromCoin.ID,
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Reverse bids so the best bid (highest price) comes first
	for i, j := 0, len(bids)-1; i < j; i, j = i+1, j-1 {
		bids[i], bids[j] = bids[j], bids[i]
	}

	// Apply limit
	if len(asks) > limit {
		asks = asks[:limit]
	}
	if len(bids) > limit {
		bids = bids[:limit]
	}

	// Convert to response levels. Data is already in nano (int64).
	askLevels := make([]schemas.OrderBookLevel, len(asks))
	for i, a := range asks {
		askLevels[i] = schemas.OrderBookLevel{
			PriceRate:   a.PriceRate,
			TotalAmount: a.TotalAmount,
			OrderCount:  a.OrderCount,
			TotalValue:  a.TotalAmount * a.PriceRate,
		}
	}

	bidLevels := make([]schemas.OrderBookLevel, len(bids))
	for i, b := range bids {
		bidLevels[i] = schemas.OrderBookLevel{
			PriceRate:   b.PriceRate,
			TotalAmount: b.TotalAmount,
			OrderCount:  b.OrderCount,
			TotalValue:  b.TotalAmount * b.PriceRate,
		}
	}

	// Compute cumulative sums
	var cumAmt, cumVal int64
	for i := range askLevels {
		cumAmt += askLevels[i].TotalAmount
		cumVal += askLevels[i].TotalValue
		askLevels[i].CumulativeAmount = cumAmt
		askLevels[i].CumulativeValue = cumVal
	}
	cumAmt, cumVal = 0, 0
	for i := range bidLevels {
		cumAmt += bidLevels[i].TotalAmount
		cumVal += bidLevels[i].TotalValue
		bidLevels[i].CumulativeAmount = cumAmt
		bidLevels[i].CumulativeValue = cumVal
	}

	// Compute spread and mid price
	var spread, midPrice *int64
	if len(askLevels) > 0 && len(bidLevels) > 0 {
		s := askLevels[0].PriceRate - bidLevels[0].PriceRate
		m := (askLevels[0].PriceRate + bidLevels[0].PriceRate) / 2
		spread = &s
		midPrice = &m
	}

	c.JSON(http.StatusOK, schemas.OrderBookResponse{
		FromSymbol:   fromCoin.Symbol,
		ToSymbol:     toCoin.Symbol,
		FromDecimals: fromCoin.Decimals,
		ToDecimals:   toCoin.Decimals,
		Spread:       spread,
		MidPrice:     midPrice,
		Asks:         askLevels,
		Bids:         bidLevels,
	})
}

