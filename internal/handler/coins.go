package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"api/internal/handler/schemas"
	"api/internal/middleware"
	"api/internal/repository"

	"github.com/gin-gonic/gin"
)

type CoinsHandler struct {
	repo repository.CoinsRepository
}

func NewCoinsHandler(repo repository.CoinsRepository) *CoinsHandler {
	return &CoinsHandler{
		repo: repo,
	}
}

// @Summary      Get coins list
// @Description  Get coins list
// @Tags         coins
// @Accept       json
// @Produce      json
// @Param        offset    query      int     false  "Offset"
// @Param        limit     query      int     false  "Limit"
// @Param        sort      query      string  false  "Sort (id, name, symbol, cnt_orders; prefix with - for desc)"
// @Param        order     query      string  false  "Order (asc, desc)"
// @Success      200   {object}	map[string]interface{}
// @Failure      400   {object}  error
// @Failure      500   {object}  error
// @Router       /coins [get]
func (h *CoinsHandler) List(c *gin.Context) {
	var coinListReq schemas.CoinListRequestHTTP
	err := c.ShouldBindQuery(&coinListReq)
	if err != nil {
		c.Set("error", err)
		// Log full error details
		fullErr := middleware.FormatErrorFull(err)
		slog.WarnContext(c.Request.Context(), "invalid query parameters",
			"error", err,
			"error_full", fullErr,
			"query", c.Request.URL.RawQuery,
		)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set defaults if not provided
	offset := int(coinListReq.Offset)
	if coinListReq.Offset == 0 {
		offset = 0
	}

	limit := int(coinListReq.Limit)
	if coinListReq.Limit == 0 {
		limit = 1000 // default limit
	}
	if limit < 1 {
		c.Set("error", "limit must be greater than 0")
		slog.WarnContext(c.Request.Context(), "invalid limit parameter", "limit", limit)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "limit must be greater than 0"})
		return
	}
	if limit > 1000 {
		limit = 1000 // max limit
	}

	sort := coinListReq.Sort
	order := coinListReq.Order

	slog.InfoContext(c.Request.Context(), "parsed request data",
		"offset", offset,
		"limit", limit,
		"sort", sort,
		"order", order,
		"request_body", coinListReq,
	)

	sortList := strings.Split(sort, ",")
	if sort == "" {
		sortList = []string{"id"}
	}
	if order == "" {
		order = "asc"
	}

	// Fetch GORM models first
	coins, err := h.repo.GetList(c.Request.Context(), offset, limit, sortList, order)
	if err != nil {
		c.Set("error", err)
		fullErr := middleware.FormatErrorFull(err)
		slog.ErrorContext(c.Request.Context(), "failed to get coins list",
			"error", err,
			"error_full", fullErr,
			"offset", offset,
			"limit", limit,
			"sort", sortList,
			"order", order,
			"request_data", map[string]interface{}{
				"offset": offset,
				"limit":  limit,
				"sort":   sortList,
				"order":  order,
			},
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"coins": coins})
}

// @Summary      Get coin by ID
// @Description  Get coin by ID
// @Tags         coins
// @Accept       json
// @Produce      json
// @Param        id    path      string  true  "Coin ID"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /coins/{id} [get]
func (h *CoinsHandler) GetByID(c *gin.Context) {
	var coinReq schemas.CoinRequestHTTP
	err := c.ShouldBindUri(&coinReq)
	if err != nil {
		c.Set("error", err)
		slog.WarnContext(c.Request.Context(), "invalid coin ID parameter", "error", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Fetch GORM model first
	coin, err := h.repo.GetByID(c.Request.Context(), coinReq.Id)
	if err != nil {
		if err.Error() == "coin not found" || err.Error() == "record not found" {
			c.Set("error", "coin not found")
			slog.InfoContext(c.Request.Context(), "coin not found", "coin_id", coinReq.Id)
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "coin not found"})
			return
		}
		c.Set("error", err)
		slog.ErrorContext(c.Request.Context(), "failed to get coin by ID",
			"error", err,
			"coin_id", coinReq.Id,
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, coin)
}
