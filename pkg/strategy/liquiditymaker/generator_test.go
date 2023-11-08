//go:build !dnum

package liquiditymaker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

func newTestMarket() types.Market {
	return types.Market{
		BaseCurrency:    "XML",
		QuoteCurrency:   "USDT",
		TickSize:        Number(0.0001),
		StepSize:        Number(0.01),
		PricePrecision:  4,
		VolumePrecision: 8,
		MinNotional:     Number(8.0),
		MinQuantity:     Number(40.0),
	}
}

func TestLiquidityOrderGenerator(t *testing.T) {
	g := &LiquidityOrderGenerator{
		Symbol: "XMLUSDT",
		Market: newTestMarket(),
	}

	scale := &bbgo.ExponentialScale{
		Domain: [2]float64{1.0, 30.0},
		Range:  [2]float64{1.0, 4.0},
	}

	err := scale.Solve()
	assert.NoError(t, err)
	assert.InDelta(t, 1.0, scale.Call(1.0), 0.00001)
	assert.InDelta(t, 4.0, scale.Call(30.0), 0.00001)

	totalAmount := Number(200_000.0)

	t.Run("ask orders", func(t *testing.T) {
		orders := g.Generate(types.SideTypeSell, totalAmount, Number(2.0), Number(2.04), 30, scale)
		assert.Len(t, orders, 30)

		totalQuoteQuantity := fixedpoint.NewFromInt(0)
		for _, o := range orders {
			totalQuoteQuantity = totalQuoteQuantity.Add(o.Quantity.Mul(o.Price))
		}
		assert.InDelta(t, totalAmount.Float64(), totalQuoteQuantity.Float64(), 1.0)

		AssertOrdersPriceSideQuantity(t, []PriceSideQuantityAssert{
			{Side: types.SideTypeSell, Price: Number("2.0000"), Quantity: Number("1513.40")},
			{Side: types.SideTypeSell, Price: Number("2.0013"), Quantity: Number("1587.50")},
			{Side: types.SideTypeSell, Price: Number("2.0027"), Quantity: Number("1665.23")},
			{Side: types.SideTypeSell, Price: Number("2.0041"), Quantity: Number("1746.77")},
			{Side: types.SideTypeSell, Price: Number("2.0055"), Quantity: Number("1832.30")},
			{Side: types.SideTypeSell, Price: Number("2.0068"), Quantity: Number("1922.02")},
			{Side: types.SideTypeSell, Price: Number("2.0082"), Quantity: Number("2016.13")},
			{Side: types.SideTypeSell, Price: Number("2.0096"), Quantity: Number("2114.85")},
			{Side: types.SideTypeSell, Price: Number("2.0110"), Quantity: Number("2218.40")},
			{Side: types.SideTypeSell, Price: Number("2.0124"), Quantity: Number("2327.02")},
			{Side: types.SideTypeSell, Price: Number("2.0137"), Quantity: Number("2440.96")},
			{Side: types.SideTypeSell, Price: Number("2.0151"), Quantity: Number("2560.48")},
			{Side: types.SideTypeSell, Price: Number("2.0165"), Quantity: Number("2685.86")},
			{Side: types.SideTypeSell, Price: Number("2.0179"), Quantity: Number("2817.37")},
			{Side: types.SideTypeSell, Price: Number("2.0193"), Quantity: Number("2955.32")},
		}, orders[0:15])

		AssertOrdersPriceSideQuantity(t, []PriceSideQuantityAssert{
			{Side: types.SideTypeSell, Price: Number("2.0386"), Quantity: Number("5771.04")},
			{Side: types.SideTypeSell, Price: Number("2.0399"), Quantity: Number("6053.62")},
		}, orders[28:30])
	})

	t.Run("bid orders", func(t *testing.T) {
		orders := g.Generate(types.SideTypeBuy, totalAmount, Number(2.0), Number(1.96), 30, scale)
		assert.Len(t, orders, 30)

		totalQuoteQuantity := fixedpoint.NewFromInt(0)
		for _, o := range orders {
			totalQuoteQuantity = totalQuoteQuantity.Add(o.Quantity.Mul(o.Price))
		}
		assert.InDelta(t, totalAmount.Float64(), totalQuoteQuantity.Float64(), 1.0)

		AssertOrdersPriceSideQuantity(t, []PriceSideQuantityAssert{
			{Side: types.SideTypeBuy, Price: Number("2.0000"), Quantity: Number("1551.37")},
			{Side: types.SideTypeBuy, Price: Number("1.9986"), Quantity: Number("1627.33")},
			{Side: types.SideTypeBuy, Price: Number("1.9972"), Quantity: Number("1707.01")},
			{Side: types.SideTypeBuy, Price: Number("1.9958"), Quantity: Number("1790.59")},
			{Side: types.SideTypeBuy, Price: Number("1.9944"), Quantity: Number("1878.27")},
			{Side: types.SideTypeBuy, Price: Number("1.9931"), Quantity: Number("1970.24")},
			{Side: types.SideTypeBuy, Price: Number("1.9917"), Quantity: Number("2066.71")},
			{Side: types.SideTypeBuy, Price: Number("1.9903"), Quantity: Number("2167.91")},
			{Side: types.SideTypeBuy, Price: Number("1.9889"), Quantity: Number("2274.06")},
			{Side: types.SideTypeBuy, Price: Number("1.9875"), Quantity: Number("2385.40")},
			{Side: types.SideTypeBuy, Price: Number("1.9862"), Quantity: Number("2502.20")},
			{Side: types.SideTypeBuy, Price: Number("1.9848"), Quantity: Number("2624.72")},
			{Side: types.SideTypeBuy, Price: Number("1.9834"), Quantity: Number("2753.24")},
			{Side: types.SideTypeBuy, Price: Number("1.9820"), Quantity: Number("2888.05")},
			{Side: types.SideTypeBuy, Price: Number("1.9806"), Quantity: Number("3029.46")},
		}, orders[0:15])

		AssertOrdersPriceSideQuantity(t, []PriceSideQuantityAssert{
			{Side: types.SideTypeBuy, Price: Number("1.9613"), Quantity: Number("5915.83")},
			{Side: types.SideTypeBuy, Price: Number("1.9600"), Quantity: Number("6205.49")},
		}, orders[28:30])
	})
}
