package bbgo

import (
	"github.com/pkg/errors"
)

var (
	ErrQuoteBalanceLevelTooLow  = errors.New("quote balance level is too low")
	ErrInsufficientQuoteBalance = errors.New("insufficient quote balance")

	ErrAssetBalanceLevelTooLow  = errors.New("asset balance level too low")
	ErrInsufficientAssetBalance = errors.New("insufficient asset balance")
	ErrAssetBalanceLevelTooHigh = errors.New("asset balance level too high")
)

// adjustQuantityByMinAmount adjusts the quantity to make the amount greater than the given minAmount
func adjustQuantityByMinAmount(quantity, currentPrice, minAmount float64) float64 {
	// modify quantity for the min amount
	amount := currentPrice * quantity
	if amount < minAmount {
		ratio := minAmount / amount
		quantity *= ratio
	}

	return quantity
}

func adjustQuantityByMaxAmount(quantity float64, price float64, maxAmount float64) float64 {
	amount := price * quantity
	if amount > maxAmount {
		ratio := maxAmount / amount
		quantity *= ratio
	}

	return quantity
}
