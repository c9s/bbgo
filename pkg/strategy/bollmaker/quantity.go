package bollmaker

import "github.com/c9s/bbgo/pkg/fixedpoint"

// QuantityOrAmount is a setting structure used for quantity/amount settings
type QuantityOrAmount struct {
	// Quantity is the base order quantity for your buy/sell order.
	// when quantity is set, the amount option will be not used.
	Quantity fixedpoint.Value `json:"quantity"`

	// Amount is the order quote amount for your buy/sell order.
	Amount fixedpoint.Value `json:"amount"`
}

// CalculateQuantity calculates the equivalent quantity of the given price when amount is set
// it returns the quantity if the quantity is set
func (qa *QuantityOrAmount) CalculateQuantity(currentPrice fixedpoint.Value) fixedpoint.Value {
	if qa.Amount > 0 {
		quantity := qa.Amount.Div(currentPrice)
		return quantity
	}

	return qa.Quantity
}
