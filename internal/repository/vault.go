package repository

import (
	dbmodels "api/internal/database/models"
	"api/internal/middleware"
	"context"
)

type VaultRepository interface {
	GetList(ctx context.Context, offset int, limit int, orderClauses []string, order string, jettonMinterAddress string, vaultType string) ([]dbmodels.Vault, error)
	GetByID(ctx context.Context, id uint64) (*dbmodels.Vault, error)
}

type vaultRepository struct {
}

func NewVaultRepository() VaultRepository {
	return &vaultRepository{}
}

func (r *vaultRepository) GetList(ctx context.Context, offset int, limit int, orderClauses []string, order string, jettonMinterAddress string, vaultType string) ([]dbmodels.Vault, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var vaults []dbmodels.Vault
	dbq := session.WithContext(ctx).Offset(offset).Limit(limit)
	if jettonMinterAddress != "" {
		dbq = dbq.Where("jetton_minter_address = ?", jettonMinterAddress)
	}
	if vaultType != "" {
		dbq = dbq.Where("vaults._type = ?::vaulttype", vaultType)
	}
	for _, clause := range orderClauses {
		dbq = dbq.Order(clause + " " + order)
	}
	stmt := dbq.Find(&vaults)
	return vaults, stmt.Error
}

func (r *vaultRepository) GetByID(ctx context.Context, id uint64) (*dbmodels.Vault, error) {
	session, err := middleware.GetDBSessionFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var vault dbmodels.Vault
	stmt := session.WithContext(ctx).Where("id = ?", id).First(&vault)
	return &vault, stmt.Error
}
