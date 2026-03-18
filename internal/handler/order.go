package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"api/internal/handler/schemas"
	"api/internal/middleware"
	"api/internal/repository"

	"github.com/gin-gonic/gin"
	addr "github.com/xssnick/tonutils-go/address"
)

type OrderHandler struct {
	repo repository.OrderRepository
}

func NewOrderHandler(repo repository.OrderRepository) *OrderHandler {
	return &OrderHandler{
		repo: repo,
	}
}

// @Summary      Get order list
// @Description  Get order list with pagination and sorting
// @Tags         orders
// @Accept       json
// @Produce      json
// @Param        offset    query      int     false  "Offset"
// @Param        limit     query      int     false  "Limit"
// @Param        sort      query      string  false  "Sort (id, created_at, deployed_at, status, type, amount, price_rate; prefix with - for desc)"
// @Param        order     query      string  false  "Order (asc, desc)"
// @Param        from_coin_id    query      int     false  "From coin ID"
// @Param        to_coin_id    query      int     false  "To coin ID"
// @Param        owner_raw_address    query      string  false  "Owner raw address"
// @Param        status    query      string  false  "Status (created, deployed, cancelled, completed, failed, pending_match, closed)"
// @Param        min_amount    query      int64     false  "Min amount"
// @Param        max_amount    query      int64     false  "Max amount"
// @Param        min_price_rate    query      int64     false  "Min price rate"
// @Param        max_price_rate    query      int64     false  "Max price rate"
// @Param        min_slippage    query      int64     false  "Min slippage"
// @Param        max_slippage    query      int64     false  "Max slippage"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /orders [get]
func (h *OrderHandler) List(c *gin.Context) {
	var orderListReq schemas.OrderListRequestHTTP
	err := c.ShouldBindQuery(&orderListReq)
	if err != nil {
		c.Set("error", err)
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
	offset := int(orderListReq.Offset)
	if orderListReq.Offset == 0 {
		offset = 0
	}

	limit := int(orderListReq.Limit)
	if orderListReq.Limit == 0 {
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

	sort := orderListReq.Sort
	order := orderListReq.Order

	slog.InfoContext(c.Request.Context(), "parsed request data",
		"offset", offset,
		"limit", limit,
		"sort", sort,
		"order", order,
		"request_body", orderListReq,
	)

	sortList := strings.Split(sort, ",")
	if sort == "" {
		sortList = []string{"id"}
	}
	if order == "" {
		order = "asc"
	}

	var filtersList []string = make([]string, 0)
	if orderListReq.FromCoinID > 0 {
		filtersList = append(filtersList, fmt.Sprintf("from_coin_id = %d", orderListReq.FromCoinID))
	}
	if orderListReq.ToCoinID > 0 {
		filtersList = append(filtersList, fmt.Sprintf("to_coin_id = %d", orderListReq.ToCoinID))
	}
	if orderListReq.Status != "" {
		filtersList = append(filtersList, fmt.Sprintf("status = '%s'", orderListReq.Status))
	}
	if orderListReq.MinAmount > 0 {
		filtersList = append(filtersList, fmt.Sprintf("amount >= %d", orderListReq.MinAmount))
	}
	if orderListReq.MaxAmount > 0 {
		filtersList = append(filtersList, fmt.Sprintf("amount <= %d", orderListReq.MaxAmount))
	}
	if orderListReq.MinPriceRate > 0 {
		filtersList = append(filtersList, fmt.Sprintf("price_rate >= %d", orderListReq.MinPriceRate))
	}
	if orderListReq.MaxPriceRate > 0 {
		filtersList = append(filtersList, fmt.Sprintf("price_rate <= %d", orderListReq.MaxPriceRate))
	}
	if orderListReq.MinSlippage > 0 {
		filtersList = append(filtersList, fmt.Sprintf("slippage >= %d", orderListReq.MinSlippage))
	}
	if orderListReq.MaxSlippage > 0 {
		filtersList = append(filtersList, fmt.Sprintf("slippage <= %d", orderListReq.MaxSlippage))
	}
	if orderListReq.OwnerRawAddress != "" {
		raw_address, err := addr.ParseRawAddr(orderListReq.OwnerRawAddress)
		if err != nil {
			c.Set("error", err)
			slog.WarnContext(c.Request.Context(), "invalid owner raw address", "error", err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		filtersList = append(filtersList, fmt.Sprintf("wallets.raw_address = '%s'", raw_address.StringRaw()))
	}

	// Fetch GORM models first
	orders, err := h.repo.GetList(c.Request.Context(), offset, limit, sortList, order, filtersList)
	if err != nil {
		c.Set("error", err)
		fullErr := middleware.FormatErrorFull(err)
		slog.ErrorContext(c.Request.Context(), "failed to get orders list",
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

	c.JSON(http.StatusOK, gin.H{"orders": orders})
}

// @Summary      Get order statistics by wallet address
// @Description  Get order counts by status (created, deployed, cancelled, completed, failed, pending_match, closed) for a wallet
// @Tags         orders
// @Accept       json
// @Produce      json
// @Param        wallet_address  query      string  true   "Wallet raw address (e.g. EQ...)"
// @Success      200   {object}  schemas.OrderStatsResponse
// @Failure      400   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /orders/stats [get]
func (h *OrderHandler) Stats(c *gin.Context) {
	var req schemas.OrderStatsRequestHTTP
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Set("error", err)
		slog.WarnContext(c.Request.Context(), "invalid query parameters", "error", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.WalletAddress == "" {
		c.Set("error", "wallet_address is required")
		slog.WarnContext(c.Request.Context(), "wallet_address is required")
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "wallet_address is required"})
		return
	}

	rawAddr, err := addr.ParseRawAddr(req.WalletAddress)
	if err != nil {
		c.Set("error", err)
		slog.WarnContext(c.Request.Context(), "invalid wallet address", "error", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	walletAddr := rawAddr.StringRaw()

	stats, total, err := h.repo.GetStatsByWalletAddress(c.Request.Context(), walletAddr)
	if err != nil {
		c.Set("error", err)
		fullErr := middleware.FormatErrorFull(err)
		slog.ErrorContext(c.Request.Context(), "failed to get order stats", "error", err, "error_full", fullErr)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	byStatus := make(map[string]int64)
	var open, closed int64
	openStatuses := map[string]bool{"created": true, "deployed": true, "pending_match": true}
	for _, s := range stats {
		byStatus[s.Status] = s.Count
		if openStatuses[s.Status] {
			open += s.Count
		} else {
			closed += s.Count
		}
	}

	c.JSON(http.StatusOK, schemas.OrderStatsResponse{
		WalletAddress: walletAddr,
		Total:         total,
		ByStatus:      byStatus,
		Open:          open,
		Closed:        closed,
	})
}

// @Summary      Get deployed order totals by token (wallet address)
// @Description  Sum of order amounts grouped by token (from_coin) for orders with status deployed
// @Tags         orders
// @Accept       json
// @Produce      json
// @Param        wallet_address  query      string  true   "Wallet raw address (e.g. EQ...)"
// @Success      200   {object}  schemas.OrderDeployedTotalsResponse
// @Failure      400   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /orders/deployed-totals [get]
func (h *OrderHandler) DeployedTotals(c *gin.Context) {
	var req schemas.OrderDeployedTotalsRequestHTTP
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Set("error", err)
		slog.WarnContext(c.Request.Context(), "invalid query parameters", "error", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.WalletAddress == "" {
		c.Set("error", "wallet_address is required")
		slog.WarnContext(c.Request.Context(), "wallet_address is required")
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "wallet_address is required"})
		return
	}

	rawAddr, err := addr.ParseRawAddr(req.WalletAddress)
	if err != nil {
		c.Set("error", err)
		slog.WarnContext(c.Request.Context(), "invalid wallet address", "error", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	walletAddr := rawAddr.StringRaw()

	rows, err := h.repo.GetDeployedTotalsByWalletAddress(c.Request.Context(), walletAddr)
	if err != nil {
		c.Set("error", err)
		fullErr := middleware.FormatErrorFull(err)
		slog.ErrorContext(c.Request.Context(), "failed to get deployed totals", "error", err, "error_full", fullErr)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totals := make([]schemas.OrderDeployedTotalsItem, 0, len(rows))
	for _, r := range rows {
		totals = append(totals, schemas.OrderDeployedTotalsItem{
			CoinID:      r.CoinID,
			Symbol:      r.Symbol,
			Name:        r.Name,
			TotalAmount: r.TotalAmount,
		})
	}

	c.JSON(http.StatusOK, schemas.OrderDeployedTotalsResponse{
		WalletAddress: walletAddr,
		Totals:        totals,
	})
}

// @Summary      Get order by ID
// @Description  Get order by ID
// @Tags         orders
// @Accept       json
// @Produce      json
// @Param        id    path      string  true  "Order ID"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]interface{}
// @Failure      404   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /orders/{id} [get]
func (h *OrderHandler) GetByID(c *gin.Context) {
	var orderReq schemas.OrderRequestHTTP
	err := c.ShouldBindUri(&orderReq)
	if err != nil {
		c.Set("error", err)
		fullErr := middleware.FormatErrorFull(err)
		slog.WarnContext(c.Request.Context(), "invalid order ID parameter",
			"error", err,
			"error_full", fullErr,
		)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id := orderReq.Id

	// Fetch GORM model first
	order, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err.Error() == "order not found" || err.Error() == "record not found" {
			c.Set("error", "order not found")
			slog.InfoContext(c.Request.Context(), "order not found", "order_id", id)
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.Set("error", err)
		fullErr := middleware.FormatErrorFull(err)
		slog.ErrorContext(c.Request.Context(), "failed to get order by ID",
			"error", err,
			"error_full", fullErr,
			"order_id", id,
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, order)
}
