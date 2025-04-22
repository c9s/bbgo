package bbgo

import (
	"github.com/c9s/bbgo/pkg/types"
)

type SymbolBasedRiskController struct {
	BasicRiskController *BasicRiskController `json:"basic,omitempty" yaml:"basic,omitempty"`
}

func groupSubmitOrdersBySymbol(orders []types.SubmitOrder) map[string][]types.SubmitOrder {
	var symbolOrders = make(map[string][]types.SubmitOrder, len(orders))
	for _, order := range orders {
		symbolOrders[order.Symbol] = append(symbolOrders[order.Symbol], order)
	}

	return symbolOrders
}
