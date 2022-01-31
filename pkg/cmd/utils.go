package cmd

import (
	"fmt"

	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/exchange/ftx"
	"github.com/c9s/bbgo/pkg/types"
)

func inQuoteAsset(balances types.BalanceMap, market types.Market, price float64) float64 {
	quote := balances[market.QuoteCurrency]
	base := balances[market.BaseCurrency]
	return base.Total().Float64()*price + quote.Total().Float64()
}

func inBaseAsset(balances types.BalanceMap, market types.Market, price float64) float64 {
	quote := balances[market.QuoteCurrency]
	base := balances[market.BaseCurrency]
	return base.Total().Float64() + (quote.Total().Float64() / price)
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
