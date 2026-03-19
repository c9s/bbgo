package ttmsqueeze

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// calculateOrderParams calculates the number of orders and slice quantity for a given total quantity
func (s *Strategy) calculateOrderParams(quantity fixedpoint.Value, numOrders int) (int, fixedpoint.Value) {
	// Calculate how many orders we can make with min quantity slices
	maxOrdersForQuantity := quantity.Div(s.Market.MinQuantity).Int()
	if maxOrdersForQuantity < 1 {
		maxOrdersForQuantity = 1
	}
	if numOrders > maxOrdersForQuantity {
		numOrders = maxOrdersForQuantity
	}

	sliceQuantity := quantity.Div(fixedpoint.NewFromInt(int64(numOrders)))
	sliceQuantity = s.Market.TruncateQuantity(sliceQuantity)

	return numOrders, sliceQuantity
}
