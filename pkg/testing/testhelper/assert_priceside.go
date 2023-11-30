package testhelper

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type PriceSideAssert struct {
	Price fixedpoint.Value
	Side  types.SideType
}

// AssertOrdersPriceSide asserts the orders with the given price and side (slice)
func AssertOrdersPriceSide(t *testing.T, asserts []PriceSideAssert, orders []types.SubmitOrder) {
	for i, a := range asserts {
		assert.Equalf(t, a.Price, orders[i].Price, "order #%d price should be %f", i+1, a.Price.Float64())
		assert.Equalf(t, a.Side, orders[i].Side, "order at price %f should be %s", a.Price.Float64(), a.Side)
	}
}

type PriceSideQuantityAssert struct {
	Price    fixedpoint.Value
	Side     types.SideType
	Quantity fixedpoint.Value
}

// AssertOrdersPriceSide asserts the orders with the given price and side (slice)
func AssertOrdersPriceSideQuantity(
	t *testing.T, asserts []PriceSideQuantityAssert, orders []types.SubmitOrder,
) {
	assert.Equalf(t, len(asserts), len(orders), "expecting %d orders", len(asserts))

	var assertPrices, orderPrices fixedpoint.Slice
	var assertPricesFloat, orderPricesFloat []float64
	for _, a := range asserts {
		assertPrices = append(assertPrices, a.Price)
		assertPricesFloat = append(assertPricesFloat, a.Price.Float64())
	}

	for _, o := range orders {
		orderPrices = append(orderPrices, o.Price)
		orderPricesFloat = append(orderPricesFloat, o.Price.Float64())
	}

	if !assert.Equalf(t, assertPricesFloat, orderPricesFloat, "assert prices") {
		return
	}

	for i, a := range asserts {
		assert.Equalf(t, a.Price.Float64(), orders[i].Price.Float64(), "order #%d price should be %f", i+1, a.Price.Float64())
		assert.Equalf(t, a.Quantity.Float64(), orders[i].Quantity.Float64(), "order #%d quantity should be %f", i+1, a.Quantity.Float64())
		assert.Equalf(t, a.Side, orders[i].Side, "order at price %f should be %s", a.Price.Float64(), a.Side)
	}
}
