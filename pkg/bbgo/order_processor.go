package bbgo

import (
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

var (
	ErrQuoteBalanceLevelTooLow  = errors.New("quote balance level is too low")
	ErrInsufficientQuoteBalance = errors.New("insufficient quote balance")

	ErrAssetBalanceLevelTooLow  = errors.New("asset balance level too low")
	ErrInsufficientAssetBalance = errors.New("insufficient asset balance")
	ErrAssetBalanceLevelTooHigh = errors.New("asset balance level too high")
)

// AdjustQuantityByMaxAmount adjusts the quantity to make the amount less than the given maxAmount
func AdjustQuantityByMaxAmount(quantity, currentPrice, maxAmount fixedpoint.Value) fixedpoint.Value {
	// modify quantity for the min amount
	amount := currentPrice.Mul(quantity)
	if amount.Compare(maxAmount) < 0 {
		return quantity
	}

	ratio := maxAmount.Div(amount)
	return quantity.Mul(ratio)
}

// AdjustQuantityByMinAmount adjusts the quantity to make the amount greater than the given minAmount
func AdjustQuantityByMinAmount(quantity, currentPrice, minAmount fixedpoint.Value) fixedpoint.Value {
	// modify quantity for the min amount
	amount := currentPrice.Mul(quantity)
	if amount.Compare(minAmount) < 0 {
		ratio := minAmount.Div(amount)
		quantity = quantity.Mul(ratio)
	}

	return quantity
}

// AdjustFloatQuantityByMinAmount adjusts the quantity to make the amount greater than the given minAmount
func AdjustFloatQuantityByMinAmount(quantity, currentPrice, minAmount fixedpoint.Value) fixedpoint.Value {
	// modify quantity for the min amount
	amount := currentPrice.Mul(quantity)
	if amount.Compare(minAmount) < 0 {
		ratio := minAmount.Div(amount)
		return quantity.Mul(ratio)
	}

	return quantity
}

func AdjustFloatQuantityByMaxAmount(quantity fixedpoint.Value, price fixedpoint.Value, maxAmount fixedpoint.Value) fixedpoint.Value {
	amount := price.Mul(quantity)
	if amount.Compare(maxAmount) > 0 {
		ratio := maxAmount.Div(amount)
		return quantity.Mul(ratio)
	}

	return quantity
}
