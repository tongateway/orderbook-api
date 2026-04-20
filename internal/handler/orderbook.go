package handler

import (
	"log/slog"
	"math/big"
	"net/http"

	"api/internal/cache"
	"api/internal/handler/schemas"
	"api/internal/middleware"
	"api/internal/repository"

	"github.com/gin-gonic/gin"
)

// bigMul computes totalAmount * priceRate using math/big and returns the result as a string.
func bigMul(amount int64, priceRateStr string) string {
	pr := new(big.Int)
	pr.SetString(priceRateStr, 10)
	return new(big.Int).Mul(big.NewInt(amount), pr).String()
}

// bigAdd adds two numeric strings using math/big.
func bigAdd(a, b string) string {
	x := new(big.Int)
	x.SetString(a, 10)
	y := new(big.Int)
	y.SetString(b, 10)
	return new(big.Int).Add(x, y).String()
}

// filterOutliers removes levels whose price is more than 100× the median
// price (or less than 1/100 of median). This drops legacy orders created
// with broken price formulas that would otherwise pollute bucketing.
//
// Input levels MUST be sorted by PriceRate ASC. Returns a filtered copy
// preserving ascending order.
func filterOutliers(levels []repository.OrderBookLevel) []repository.OrderBookLevel {
	if len(levels) < 3 {
		return levels
	}

	// Find weighted-by-amount median price (fall back to position median if no amounts).
	// Pre-parse all prices once.
	prices := make([]*big.Int, len(levels))
	for i, lv := range levels {
		p := new(big.Int)
		p.SetString(lv.PriceRate, 10)
		prices[i] = p
	}

	// Levels are sorted ASC; find the level whose cumulative amount crosses 50%.
	var totalAmt int64
	for _, lv := range levels {
		totalAmt += lv.TotalAmount
	}
	var median *big.Int
	if totalAmt > 0 {
		halfAmt := totalAmt / 2
		var cum int64
		for i, lv := range levels {
			cum += lv.TotalAmount
			if cum >= halfAmt {
				median = prices[i]
				break
			}
		}
	}
	if median == nil {
		// Fallback: positional median
		median = prices[len(prices)/2]
	}
	if median.Sign() == 0 {
		return levels
	}

	// Keep levels within [median/100, median*100]
	lower := new(big.Int).Div(median, big.NewInt(100))
	upper := new(big.Int).Mul(median, big.NewInt(100))

	out := make([]repository.OrderBookLevel, 0, len(levels))
	for i, lv := range levels {
		if prices[i].Cmp(lower) >= 0 && prices[i].Cmp(upper) <= 0 {
			out = append(out, lv)
		}
	}
	return out
}

// aggregateLevels buckets raw price levels into at most `limit` levels.
// If len(levels) <= limit, returns levels unchanged.
// Otherwise computes an automatic tick size from the price range and merges
// levels that fall into the same bucket.
// Input levels MUST be sorted by PriceRate ASC.
func aggregateLevels(levels []repository.OrderBookLevel, limit int) []repository.OrderBookLevel {
	if len(levels) <= limit || limit <= 0 {
		return levels
	}

	// Parse all prices as big.Int
	prices := make([]*big.Int, len(levels))
	for i, lv := range levels {
		p := new(big.Int)
		p.SetString(lv.PriceRate, 10)
		prices[i] = p
	}

	minPrice := prices[0]
	maxPrice := prices[len(prices)-1]

	// tick = (maxPrice - minPrice) / limit
	rangeVal := new(big.Int).Sub(maxPrice, minPrice)
	tick := new(big.Int).Div(rangeVal, big.NewInt(int64(limit)))

	// If tick is 0 (all prices are the same or very close), return as-is
	if tick.Sign() == 0 {
		if len(levels) > limit {
			return levels[:limit]
		}
		return levels
	}

	// Bucket levels: bucketIndex = (price - minPrice) / tick
	type bucket struct {
		sumPrice    *big.Int // sum of prices weighted by amount, for midpoint calc
		sumAmount   int64
		totalAmount int64
		orderCount  int64
		count       int // number of levels merged
	}

	buckets := make(map[int64]*bucket)
	var bucketOrder []int64 // preserve order

	for i, lv := range levels {
		idx := new(big.Int).Sub(prices[i], minPrice)
		idx.Div(idx, tick)
		bucketIdx := idx.Int64()

		b, exists := buckets[bucketIdx]
		if !exists {
			b = &bucket{sumPrice: new(big.Int)}
			buckets[bucketIdx] = b
			bucketOrder = append(bucketOrder, bucketIdx)
		}

		// Weighted price: add price * amount for weighted average
		// If amount is 0, just add the price itself (count-based average)
		if lv.TotalAmount > 0 {
			weighted := new(big.Int).Mul(prices[i], big.NewInt(lv.TotalAmount))
			b.sumPrice.Add(b.sumPrice, weighted)
			b.sumAmount += lv.TotalAmount
		} else {
			b.sumPrice.Add(b.sumPrice, prices[i])
			b.count++
		}
		b.totalAmount += lv.TotalAmount
		b.orderCount += lv.OrderCount
	}

	// Build aggregated levels
	result := make([]repository.OrderBookLevel, 0, len(bucketOrder))
	for _, idx := range bucketOrder {
		b := buckets[idx]

		// Compute representative price: weighted average by amount
		var reprPrice *big.Int
		if b.sumAmount > 0 {
			reprPrice = new(big.Int).Div(b.sumPrice, big.NewInt(b.sumAmount))
		} else if b.count > 0 {
			reprPrice = new(big.Int).Div(b.sumPrice, big.NewInt(int64(b.count)))
		} else {
			// Fallback: bucket midpoint
			reprPrice = new(big.Int).Mul(big.NewInt(idx), tick)
			reprPrice.Add(reprPrice, minPrice)
			halfTick := new(big.Int).Div(tick, big.NewInt(2))
			reprPrice.Add(reprPrice, halfTick)
		}

		result = append(result, repository.OrderBookLevel{
			PriceRate:   reprPrice.String(),
			TotalAmount: b.totalAmount,
			OrderCount:  b.orderCount,
		})
	}

	return result
}

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
// @Param        limit              query  int     false  "Max number of price levels per side (default 10, max 50). When more unique prices exist, levels are automatically aggregated into buckets."
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
	if limit > 50 {
		limit = 50
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

	// Drop anomalous outlier prices (legacy orders with broken formula) so
	// bucketing doesn't get polluted. Then aggregate to `limit` levels.
	// Both asks and bids are sorted ASC from cache.
	asks = filterOutliers(asks)
	bids = filterOutliers(bids)
	asks = aggregateLevels(asks, limit)
	bids = aggregateLevels(bids, limit)

	// Reverse bids so the best bid (highest price) comes first
	for i, j := 0, len(bids)-1; i < j; i, j = i+1, j-1 {
		bids[i], bids[j] = bids[j], bids[i]
	}

	// Convert to response levels. PriceRate is now a string (numeric).
	askLevels := make([]schemas.OrderBookLevel, len(asks))
	for i, a := range asks {
		askLevels[i] = schemas.OrderBookLevel{
			PriceRate:   a.PriceRate,
			TotalAmount: a.TotalAmount,
			OrderCount:  a.OrderCount,
			TotalValue:  bigMul(a.TotalAmount, a.PriceRate),
		}
	}

	bidLevels := make([]schemas.OrderBookLevel, len(bids))
	for i, b := range bids {
		bidLevels[i] = schemas.OrderBookLevel{
			PriceRate:   b.PriceRate,
			TotalAmount: b.TotalAmount,
			OrderCount:  b.OrderCount,
			TotalValue:  bigMul(b.TotalAmount, b.PriceRate),
		}
	}

	// Compute cumulative sums
	var cumAmt int64
	cumVal := "0"
	for i := range askLevels {
		cumAmt += askLevels[i].TotalAmount
		cumVal = bigAdd(cumVal, askLevels[i].TotalValue)
		askLevels[i].CumulativeAmount = cumAmt
		askLevels[i].CumulativeValue = cumVal
	}
	cumAmt = 0
	cumVal = "0"
	for i := range bidLevels {
		cumAmt += bidLevels[i].TotalAmount
		cumVal = bigAdd(cumVal, bidLevels[i].TotalValue)
		bidLevels[i].CumulativeAmount = cumAmt
		bidLevels[i].CumulativeValue = cumVal
	}

	// Compute spread and mid price using math/big
	var spread, midPrice *string
	if len(askLevels) > 0 && len(bidLevels) > 0 {
		askPR := new(big.Int)
		askPR.SetString(askLevels[0].PriceRate, 10)
		bidPR := new(big.Int)
		bidPR.SetString(bidLevels[0].PriceRate, 10)
		s := new(big.Int).Sub(askPR, bidPR).String()
		m := new(big.Int).Add(askPR, bidPR)
		m.Div(m, big.NewInt(2))
		mStr := m.String()
		spread = &s
		midPrice = &mStr
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

