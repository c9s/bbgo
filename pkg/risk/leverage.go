package risk

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// How to Calculate Cost Required to Open a Position in Perpetual Futures Contracts
//
// See <https://www.binance.com/en/support/faq/87fa7ee33b574f7084d42bd2ce2e463b>
//
// For Long Position:
// = Number of Contract * Absolute Value {min[0, direction of order x (mark price - order price)]}
//
// For short position:
// = Number of Contract * Absolute Value {min[0, direction of order x (mark price - order price)]}
func CalculateOpenLoss(numContract, markPrice, orderPrice fixedpoint.Value, side types.SideType) fixedpoint.Value {
	var d = fixedpoint.One
	if side == types.SideTypeSell {
		d = fixedpoint.NegOne
	}

	var openLoss = numContract.Mul(fixedpoint.Min(fixedpoint.Zero, d.Mul(markPrice.Sub(orderPrice))).Abs())
	return openLoss
}

// CalculateMarginCost calculate the margin cost of the given notional position by price * quantity
func CalculateMarginCost(price, quantity, leverage fixedpoint.Value) fixedpoint.Value {
	var notionalValue = price.Mul(quantity)
	var cost = notionalValue.Div(leverage)
	return cost
}

func CalculatePositionCost(markPrice, orderPrice, quantity, leverage fixedpoint.Value, side types.SideType) fixedpoint.Value {
	var marginCost = CalculateMarginCost(orderPrice, quantity, leverage)
	var openLoss = CalculateOpenLoss(quantity, markPrice, orderPrice, side)
	return marginCost.Add(openLoss)
}

// CalculateMaxPosition calculates the maximum notional value of the position and return the max quantity you can use.
func CalculateMaxPosition(price, availableMargin, leverage fixedpoint.Value) fixedpoint.Value {
	var maxNotionalValue = availableMargin.Mul(leverage)
	var maxQuantity = maxNotionalValue.Div(price)
	return maxQuantity
}

// CalculateMinRequiredLeverage calculates the leverage of the given position (price and quantity)
func CalculateMinRequiredLeverage(price, quantity, availableMargin fixedpoint.Value) fixedpoint.Value {
	var notional = price.Mul(quantity)
	return notional.Div(availableMargin)
}
