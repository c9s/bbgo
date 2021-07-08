package bbgo

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/pkg/errors"
)

var (
	ErrQuoteBalanceLevelTooLow  = errors.New("quote balance level is too low")
	ErrInsufficientQuoteBalance = errors.New("insufficient quote balance")

	ErrAssetBalanceLevelTooLow  = errors.New("asset balance level too low")
	ErrInsufficientAssetBalance = errors.New("insufficient asset balance")
	ErrAssetBalanceLevelTooHigh = errors.New("asset balance level too high")
)

// AdjustQuantityByMaxAmount adjusts the quantity to make the amount greater than the given minAmount
func AdjustQuantityByMaxAmount(quantity, currentPrice, maxAmount fixedpoint.Value) fixedpoint.Value {
	// modify quantity for the min amount
	amount := currentPrice.Mul(quantity)
	if amount < maxAmount {
		return quantity
	}

	ratio := maxAmount.Div(amount)
	return quantity.Mul(ratio)
}

// AdjustQuantityByMinAmount adjusts the quantity to make the amount greater than the given minAmount
func AdjustQuantityByMinAmount(quantity, currentPrice, minAmount fixedpoint.Value) fixedpoint.Value {
	// modify quantity for the min amount
	amount := currentPrice.Mul(quantity)
	if amount < minAmount {
		ratio := minAmount.Div(amount)
		quantity = quantity.Mul(ratio)
	}

	return quantity
}

// AdjustFloatQuantityByMinAmount adjusts the quantity to make the amount greater than the given minAmount
func AdjustFloatQuantityByMinAmount(quantity, currentPrice, minAmount float64) float64 {
	// modify quantity for the min amount
	amount := currentPrice * quantity
	if amount < minAmount {
		ratio := minAmount / amount
		quantity *= ratio
	}

	return quantity
}

func AdjustFloatQuantityByMaxAmount(quantity float64, price float64, maxAmount float64) float64 {
	amount := price * quantity
	if amount > maxAmount {
		ratio := maxAmount / amount
		quantity *= ratio
	}

	return quantity
}
