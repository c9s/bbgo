package testhelper

import (
	"fmt"
	"strings"
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

func ParsePriceSideQuantityAssertions(text string) []PriceSideQuantityAssert {
	var asserts []PriceSideQuantityAssert
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		cols := strings.SplitN(line, ",", 3)
		if len(cols) < 3 {
			panic(fmt.Errorf("column length should be 3, got %d", len(cols)))
		}

		side := strings.TrimSpace(cols[0])
		price := fixedpoint.MustNewFromString(strings.TrimSpace(cols[1]))
		quantity := fixedpoint.MustNewFromString(strings.TrimSpace(cols[2]))
		asserts = append(asserts, PriceSideQuantityAssert{
			Price:    price,
			Side:     types.SideType(side),
			Quantity: quantity,
		})
	}

	return asserts
}

func AssertOrdersPriceSideQuantityFromText(
	t *testing.T, text string, orders []types.SubmitOrder,
) {
	asserts := ParsePriceSideQuantityAssertions(text)
	assert.Equalf(t, len(asserts), len(orders), "expecting %d orders", len(asserts))
	for i, a := range asserts {
		order := orders[i]
		assert.Equalf(t, a.Price.Float64(), order.Price.Float64(), "order #%d price should be %f", i+1, a.Price.Float64())
		assert.Equalf(t, a.Quantity.Float64(), order.Quantity.Float64(), "order #%d quantity should be %f", i+1, a.Quantity.Float64())
		assert.Equalf(t, a.Side, orders[i].Side, "order at price %f should be %s", a.Price.Float64(), a.Side)

	}

	if t.Failed() {
		actualInText := "Actual Orders:\n"
		for _, order := range orders {
			actualInText += fmt.Sprintf("%s,%s,%s\n", order.Side, order.Price.String(), order.Quantity.String())
		}
		t.Log(actualInText)
	}
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
