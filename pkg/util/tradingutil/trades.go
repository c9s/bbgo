package tradingutil

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// CollectTradeFee collects the fee from the given trade slice
func CollectTradeFee(trades []types.Trade) map[string]fixedpoint.Value {
	fees := make(map[string]fixedpoint.Value)
	for _, t := range trades {
		if fee, ok := fees[t.FeeCurrency]; ok {
			fees[t.FeeCurrency] = fee.Add(t.Fee)
		} else {
			fees[t.FeeCurrency] = t.Fee
		}
	}

	return fees
}

func AggregateTradesQuantity(trades []types.Trade) fixedpoint.Value {
	tq := fixedpoint.Zero
	for _, t := range trades {
		tq = tq.Add(t.Quantity)
	}
	return tq
}
