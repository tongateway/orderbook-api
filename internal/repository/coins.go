package repository

import (
	"context"

	dbmodels "api/internal/database/models"
	"api/internal/middleware"
)

type CoinsRepository interface {
	GetList(ctx context.Context, offset int, limit int, orderClauses []string, order string) ([]dbmodels.Coin, error)
	GetByID(ctx context.Context, id uint64) (*dbmodels.Coin, error)
	GetBySymbol(ctx context.Context, symbol string) (*dbmodels.Coin, error)
	GetByTonRawAddress(ctx context.Context, tonRawAddress string) (*dbmodels.Coin, error)
}

type coinsRepository struct {
}

func NewCoinsRepository() CoinsRepository {
	return &coinsRepository{}
}

func (r *coinsRepository) GetList(ctx context.Context, offset int, limit int, orderClauses []string, order string) ([]dbmodels.Coin, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var coins []dbmodels.Coin
	dbq := session.WithContext(ctx).Offset(offset).Limit(limit)
	for _, clause := range orderClauses {
		if clause == "cnt_orders" {
			dbq = dbq.Joins("LEFT JOIN orders ON coins.id = orders.from_coin_id")
			dbq = dbq.Order("COUNT(orders.id) " + order)
		} else {
			dbq = dbq.Order(clause + " " + order)
		}
	}
	stmt := dbq.Group("coins.id").Find(&coins)
	return coins, stmt.Error
}

func (r *coinsRepository) GetByID(ctx context.Context, id uint64) (*dbmodels.Coin, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var coin dbmodels.Coin
	stmt := session.WithContext(ctx).Where("id = ?", id).First(&coin)
	return &coin, stmt.Error
}

// GetBySymbol returns the first coin matching the given symbol (case-insensitive)
func (r *coinsRepository) GetBySymbol(ctx context.Context, symbol string) (*dbmodels.Coin, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var coin dbmodels.Coin
	stmt := session.WithContext(ctx).Where("LOWER(symbol) = LOWER(?)", symbol).First(&coin)
	return &coin, stmt.Error
}

// GetByTonRawAddress returns the coin with the given jetton minter address
func (r *coinsRepository) GetByTonRawAddress(ctx context.Context, tonRawAddress string) (*dbmodels.Coin, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var coin dbmodels.Coin
	stmt := session.WithContext(ctx).Where("ton_raw_address = ?", tonRawAddress).First(&coin)
	return &coin, stmt.Error
}
