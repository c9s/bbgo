package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func cobraInitRequired(required []string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		for _, key := range required {
			if err := cmd.MarkFlagRequired(key); err != nil {
				log.WithError(err).Errorf("cannot mark --%s option required", key)
			}
		}
		return nil
	}
}

// inQuoteAsset converts all balances in quote asset
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
