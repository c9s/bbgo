package tri

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

func fitQuantityByBase(quantity, balance float64) (float64, float64) {
	q := math.Min(quantity, balance)
	r := q / balance
	return q, r
}

// 1620 x 2 , quote balance = 1000 => rate = 1000/(1620*2) = 0.3086419753, quantity = 0.61728395
func fitQuantityByQuote(price, quantity, quoteBalance float64) (float64, float64) {
	quote := quantity * price
	minQuote := math.Min(quote, quoteBalance)
	q := minQuote / price
	r := minQuote / quoteBalance
	return q, r
}

func logSubmitOrders(orders [3]types.SubmitOrder) {
	for i, order := range orders {
		log.Infof("SUBMIT ORDER #%d: %s", i, order.String())
	}
}
