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

type VaultHandler struct {
	repo repository.VaultRepository
}

func NewVaultHandler(repo repository.VaultRepository) *VaultHandler {
	return &VaultHandler{
		repo: repo,
	}
}

// @Summary      Get vaults list
// @Description  Get vaults list
// @Tags         vaults
// @Accept       json
// @Produce      json
// @Param        offset    query      int     false  "Offset"
// @Param        limit     query      int     false  "Limit"
// @Param        sort      query      string  false  "Sort (id, factory_id, created_at, type; prefix with - for desc)"
// @Param        order     query      string  false  "Order (asc, desc)"
// @Param        jetton_minter_address  query  string  false  "Filter by jetton minter address"
// @Param        type                   query  string  false  "Filter by type (jetton, ton)"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  error
// @Failure      500   {object}  error
// @Router       /vaults [get]
func (h *VaultHandler) List(c *gin.Context) {
	var vaultListReq schemas.VaultListRequestHTTP
	err := c.ShouldBindQuery(&vaultListReq)
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
	offset := int(vaultListReq.Offset)
	if vaultListReq.Offset == 0 {
		offset = 0
	}

	limit := int(vaultListReq.Limit)
	if vaultListReq.Limit == 0 {
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

	sort := vaultListReq.Sort
	order := vaultListReq.Order

	slog.InfoContext(c.Request.Context(), "parsed request data",
		"offset", offset,
		"limit", limit,
		"sort", sort,
		"order", order,
		"request_body", vaultListReq,
	)

	sortList := strings.Split(sort, ",")
	if sort == "" {
		sortList = []string{"id"}
	}
	if order == "" {
		order = "asc"
	}

	// Fetch GORM models first
	vaults, err := h.repo.GetList(c.Request.Context(), offset, limit, sortList, order, vaultListReq.JettonMinterAddress, vaultListReq.Type)
	if err != nil {
		c.Set("error", err)
		fullErr := middleware.FormatErrorFull(err)
		slog.ErrorContext(c.Request.Context(), "failed to get vaults list",
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

	c.JSON(http.StatusOK, gin.H{"vaults": vaults})
}

// @Summary      Get vault by ID
// @Description  Get vault by ID
// @Tags         vaults
// @Accept       json
// @Produce      json
// @Param        id    path      string  true  "Vault ID"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]interface{}
// @Failure      404   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /vaults/{id} [get]
func (h *VaultHandler) GetByID(c *gin.Context) {
	var vaultReq schemas.VaultRequestHTTP
	err := c.ShouldBindUri(&vaultReq)
	if err != nil {
		c.Set("error", err)
		slog.WarnContext(c.Request.Context(), "invalid vault ID parameter", "error", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Fetch GORM model first
	vault, err := h.repo.GetByID(c.Request.Context(), vaultReq.Id)
	if err != nil {
		if err.Error() == "vault not found" || err.Error() == "record not found" {
			c.Set("error", "vault not found")
			slog.InfoContext(c.Request.Context(), "vault not found", "vault_id", vaultReq.Id)
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "vault not found"})
			return
		}
		c.Set("error", err)
		fullErr := middleware.FormatErrorFull(err)
		slog.ErrorContext(c.Request.Context(), "failed to get vault by ID",
			"error", err,
			"error_full", fullErr,
			"vault_id", vaultReq.Id,
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, vault)
}
