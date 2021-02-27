package cmd

import (
	"fmt"

	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/exchange/ftx"
	"github.com/c9s/bbgo/pkg/types"
)

func inBaseAsset(balances types.BalanceMap, market types.Market, price float64) float64 {
	quote := balances[market.QuoteCurrency]
	base := balances[market.BaseCurrency]
	return (base.Locked.Float64() + base.Available.Float64()) + ((quote.Locked.Float64() + quote.Available.Float64()) / price)
}

func newExchange(session string) (types.Exchange, error) {
	switch session {
	case "ftx":
		return ftx.NewExchange(
			viper.GetString("ftx-api-key"),
			viper.GetString("ftx-api-secret"),
			viper.GetString("ftx-subaccount-name"),
		), nil

	}
	return nil, fmt.Errorf("unsupported session %s", session)
}
