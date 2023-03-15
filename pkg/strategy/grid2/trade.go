package grid2

import (
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// collectTradeFee collects the fee from the given trade slice
func collectTradeFee(trades []types.Trade) map[string]fixedpoint.Value {
	fees := make(map[string]fixedpoint.Value)
	for _, t := range trades {
		feeCurrency := strings.ToUpper(t.FeeCurrency)
		if fee, ok := fees[feeCurrency]; ok {
			fees[feeCurrency] = fee.Add(t.Fee)
		} else {
			fees[feeCurrency] = t.Fee
		}
	}
	return fees
}

func aggregateTradesQuantity(trades []types.Trade) fixedpoint.Value {
	tq := fixedpoint.Zero
	for _, t := range trades {
		tq = tq.Add(t.Quantity)
	}
	return tq
}
