package handler

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"

	"api/internal/cache"
	dbmodels "api/internal/database/models"
	"api/internal/handler/schemas"
	"api/internal/middleware"
	"api/internal/repository"

	"github.com/gin-gonic/gin"
)

type CoinPriceHandler struct {
	cache     *cache.CoinPriceCache
	coinsRepo repository.CoinsRepository
}

func NewCoinPriceHandler(c *cache.CoinPriceCache, coinsRepo repository.CoinsRepository) *CoinPriceHandler {
	return &CoinPriceHandler{
		cache:     c,
		coinsRepo: coinsRepo,
	}
}

// @Summary      Get coin price from order book
// @Description  Returns order book price summary for a coin across all its trading pairs.
// @Description  Coin can be identified by name, symbol, or jetton minter address.
// @Description  Returns best ask, best bid, mid-price, and spread for each pair.
// @Description  Response is cached for 30 seconds.
// @Tags         coins
// @Accept       json
// @Produce      json
// @Param        name          query  string  false  "Coin name (e.g. AgentM)"
// @Param        symbol        query  string  false  "Coin symbol (e.g. AGENTM)"
// @Param        jetton_minter query  string  false  "Jetton minter address"
// @Success      200  {object}  schemas.CoinPriceResponse
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /coins/price [get]
func (h *CoinPriceHandler) GetPrice(c *gin.Context) {
	var req schemas.CoinPriceRequestHTTP
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Set("error", err)
		slog.WarnContext(c.Request.Context(), "invalid coin price query parameters", "error", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hasName := req.Name != ""
	hasSymbol := req.Symbol != ""
	hasMinter := req.JettonMinter != ""

	// Exactly one identifier must be provided
	identifiers := 0
	if hasName {
		identifiers++
	}
	if hasSymbol {
		identifiers++
	}
	if hasMinter {
		identifiers++
	}

	if identifiers == 0 {
		c.Set("error", "coin not specified")
		slog.WarnContext(c.Request.Context(), "coin not specified", "query", c.Request.URL.RawQuery)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "specify coin using name, symbol, or jetton_minter",
		})
		return
	}

	if identifiers > 1 {
		c.Set("error", "ambiguous coin specification")
		slog.WarnContext(c.Request.Context(), "multiple coin identifiers provided")
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "use only one of: name, symbol, or jetton_minter",
		})
		return
	}

	ctx := c.Request.Context()
	var coin coinInfo
	var coinName string
	var err error

	switch {
	case hasName:
		// Check for TON by name
		if strings.EqualFold(req.Name, "TON") || strings.EqualFold(req.Name, "Toncoin") {
			coin = coinInfo{ID: coinIDTON, Symbol: "TON", Decimals: tonDecimals}
			coinName = "Toncoin"
		} else {
			dbCoin, lookupErr := h.coinsRepo.GetByName(ctx, req.Name)
			if lookupErr != nil {
				c.Set("error", lookupErr)
				slog.WarnContext(ctx, "coin not found by name", "name", req.Name, "error", lookupErr)
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("coin with name '%s' not found", req.Name)})
				return
			}
			coin = coinInfoFromDB(dbCoin)
			if dbCoin.Name != nil {
				coinName = *dbCoin.Name
			}
		}
	case hasSymbol:
		coin, err = resolveCoinBySymbol(ctx, h.coinsRepo, req.Symbol)
		if err != nil {
			c.Set("error", err)
			slog.WarnContext(ctx, "failed to resolve symbol", "symbol", req.Symbol, "error", err)
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		coinName = coin.Symbol
	case hasMinter:
		coin, err = resolveCoinByMinter(ctx, h.coinsRepo, req.JettonMinter)
		if err != nil {
			c.Set("error", err)
			slog.WarnContext(ctx, "failed to resolve jetton minter", "minter", req.JettonMinter, "error", err)
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		coinName = coin.Symbol
	}

	// Get price summary from cache
	rows, err := h.cache.Get(ctx, coin.ID)
	if err != nil {
		c.Set("error", err)
		fullErr := middleware.FormatErrorFull(err)
		slog.ErrorContext(ctx, "failed to get coin price summary",
			"error", err,
			"error_full", fullErr,
			"coin_id", coin.ID,
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Group rows by counter_coin_id to merge ask/bid sides
	type pairData struct {
		askBestPrice   *string
		bidBestPrice   *string
		askTotalAmount int64
		bidTotalAmount int64
		askOrderCount  int64
		bidOrderCount  int64
	}
	pairMap := make(map[int]*pairData)

	for _, row := range rows {
		pd, ok := pairMap[row.CounterCoinID]
		if !ok {
			pd = &pairData{}
			pairMap[row.CounterCoinID] = pd
		}
		price := row.BestPrice
		if row.Side == "ask" {
			pd.askBestPrice = &price
			pd.askTotalAmount = row.TotalAmount
			pd.askOrderCount = row.OrderCount
		} else {
			pd.bidBestPrice = &price
			pd.bidTotalAmount = row.TotalAmount
			pd.bidOrderCount = row.OrderCount
		}
	}

	// Build response pairs with counter-coin info
	pairs := make([]schemas.CoinPricePair, 0, len(pairMap))
	for counterCoinID, pd := range pairMap {
		counterInfo, resolveErr := h.resolveCounterCoin(ctx, counterCoinID)
		if resolveErr != nil {
			slog.WarnContext(ctx, "failed to resolve counter coin",
				"counter_coin_id", counterCoinID, "error", resolveErr)
			continue
		}

		pair := schemas.CoinPricePair{
			CounterCoinID:       counterCoinID,
			CounterCoinSymbol:   counterInfo.Symbol,
			CounterCoinDecimals: counterInfo.Decimals,
			BestAsk:             pd.askBestPrice,
			BestBid:             pd.bidBestPrice,
			AskTotalAmount:      pd.askTotalAmount,
			BidTotalAmount:      pd.bidTotalAmount,
			AskOrderCount:       pd.askOrderCount,
			BidOrderCount:       pd.bidOrderCount,
		}

		// Compute mid-price and spread if both sides exist
		if pd.askBestPrice != nil && pd.bidBestPrice != nil {
			askBig := new(big.Int)
			askBig.SetString(*pd.askBestPrice, 10)
			bidBig := new(big.Int)
			bidBig.SetString(*pd.bidBestPrice, 10)
			mid := new(big.Int).Add(askBig, bidBig)
			mid.Div(mid, big.NewInt(2))
			spread := new(big.Int).Sub(askBig, bidBig)
			midStr := mid.String()
			spreadStr := spread.String()
			pair.MidPrice = &midStr
			pair.Spread = &spreadStr
		}

		pairs = append(pairs, pair)
	}

	c.JSON(http.StatusOK, schemas.CoinPriceResponse{
		Coin: schemas.CoinPriceCoinInfo{
			ID:       coin.ID,
			Name:     coinName,
			Symbol:   coin.Symbol,
			Decimals: coin.Decimals,
		},
		Pairs: pairs,
	})
}

// resolveCounterCoin resolves a counter coin ID to its info.
func (h *CoinPriceHandler) resolveCounterCoin(ctx context.Context, coinID int) (coinInfo, error) {
	if coinID == coinIDTON {
		return coinInfo{ID: coinIDTON, Symbol: "TON", Decimals: tonDecimals}, nil
	}
	// Audit 05-H1 (gosec G115): reject non-positive coin IDs before the
	// int → uint64 cast. Negative values would wrap to huge numbers and
	// cause confusing "not found" errors instead of explicit rejection.
	if coinID < 0 {
		return coinInfo{}, fmt.Errorf("invalid coin id: %d", coinID)
	}

	dbCoin, err := h.coinsRepo.GetByID(ctx, uint64(coinID))
	if err != nil {
		return coinInfo{}, err
	}
	return coinInfoFromDB(dbCoin), nil
}

// coinInfoFromDB converts a DB coin model to coinInfo.
func coinInfoFromDB(dbCoin *dbmodels.Coin) coinInfo {
	info := coinInfo{ID: dbCoin.ID, Decimals: tonDecimals}
	if dbCoin.Symbol != nil {
		info.Symbol = *dbCoin.Symbol
	}
	if dbCoin.Decimals != nil {
		info.Decimals = *dbCoin.Decimals
	}
	return info
}
