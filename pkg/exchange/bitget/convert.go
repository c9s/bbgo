package bitget

import (
	"strings"

	"github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalSymbol(s string) string {
	return strings.ToUpper(s)
}

func toGlobalBalance(asset bitgetapi.AccountAsset) types.Balance {
	return types.Balance{
		Currency:          asset.CoinName,
		Available:         asset.Available,
		Locked:            asset.Lock.Add(asset.Frozen),
		Borrowed:          fixedpoint.Zero,
		Interest:          fixedpoint.Zero,
		NetAsset:          fixedpoint.Zero,
		MaxWithdrawAmount: fixedpoint.Zero,
	}
}
