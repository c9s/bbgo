package cmd

import (
	"fmt"

	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/exchange/ftx"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func inQuoteAsset(balances types.BalanceMap, market types.Market, price fixedpoint.Value) fixedpoint.Value {
	quote := balances[market.QuoteCurrency]
	base := balances[market.BaseCurrency]
	return base.Total().Mul(price).Add(quote.Total())
}

func inBaseAsset(balances types.BalanceMap, market types.Market, price fixedpoint.Value) fixedpoint.Value {
	quote := balances[market.QuoteCurrency]
	base := balances[market.BaseCurrency]
	return quote.Total().Div(price).Add(base.Total())
}

func newExchange(session string) (types.Exchange, error) {
	switch session {
	case "ftx":
		return ftx.NewExchange(
			viper.GetString("ftx-api-key"),
			viper.GetString("ftx-api-secret"),
			viper.GetString("ftx-subaccount"),
		), nil

	}
	return nil, fmt.Errorf("unsupported session %s", session)
}
