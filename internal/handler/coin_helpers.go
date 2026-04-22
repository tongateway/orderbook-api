package handler

import (
	"context"
	"fmt"
	"strings"

	"api/internal/repository"
)

// coinIDTON is the sentinel value for native TON (NULL coin_id in orders table).
const coinIDTON = 0

// tonDecimals is the number of decimals for native TON.
const tonDecimals = 9

// coinInfo holds resolved coin identification.
type coinInfo struct {
	ID       int
	Symbol   string
	Decimals int
}

// normalizeSymbol maps known aliases to canonical DB symbols.
func normalizeSymbol(symbol string) string {
	if strings.EqualFold(symbol, "jusdt") {
		return "usdt"
	}
	return symbol
}

// isStablecoin returns true if the coin's symbol matches a known USD stablecoin.
// Matching is case-insensitive and tolerates the Unicode tether variant "USD₮".
func isStablecoin(symbol string) bool {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	switch s {
	case "USDT", "USDC", "USD₮":
		return true
	}
	return false
}

// resolveCoinBySymbol resolves a coin symbol to coinInfo.
// Returns coinID=0, decimals=9 for native TON.
func resolveCoinBySymbol(ctx context.Context, coinsRepo repository.CoinsRepository, symbol string) (coinInfo, error) {
	if strings.EqualFold(symbol, "TON") {
		return coinInfo{ID: coinIDTON, Symbol: "TON", Decimals: tonDecimals}, nil
	}

	symbol = normalizeSymbol(symbol)

	coin, err := coinsRepo.GetBySymbol(ctx, symbol)
	if err != nil {
		return coinInfo{}, fmt.Errorf("coin with symbol '%s' not found", symbol)
	}

	info := coinInfo{ID: coin.ID, Decimals: tonDecimals}
	if coin.Symbol != nil {
		info.Symbol = *coin.Symbol
	}
	if coin.Decimals != nil {
		info.Decimals = *coin.Decimals
	}
	return info, nil
}

// resolveCoinByMinter resolves a jetton minter address to coinInfo.
// Returns coinID=0, decimals=9 for native TON when minter is "ton".
func resolveCoinByMinter(ctx context.Context, coinsRepo repository.CoinsRepository, minter string) (coinInfo, error) {
	if strings.EqualFold(minter, "ton") {
		return coinInfo{ID: coinIDTON, Symbol: "TON", Decimals: tonDecimals}, nil
	}

	coin, err := coinsRepo.GetByTonRawAddress(ctx, minter)
	if err != nil {
		return coinInfo{}, fmt.Errorf("coin with jetton minter '%s' not found", minter)
	}

	info := coinInfo{ID: coin.ID, Decimals: tonDecimals}
	if coin.Symbol != nil {
		info.Symbol = *coin.Symbol
	}
	if coin.Decimals != nil {
		info.Decimals = *coin.Decimals
	}
	return info, nil
}
