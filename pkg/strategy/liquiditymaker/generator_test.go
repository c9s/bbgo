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

	totalAmount := Number(20_000.0)

	t.Run("ask orders", func(t *testing.T) {
		orders := g.Generate(types.SideTypeSell, totalAmount, Number(2.0), Number(2.04), 30, scale)
		assert.Len(t, orders, 30)

		totalQuoteQuantity := fixedpoint.NewFromInt(0)
		for _, o := range orders {
			totalQuoteQuantity = totalQuoteQuantity.Add(o.Quantity.Mul(o.Price))
		}
		assert.InDelta(t, totalAmount.Float64(), totalQuoteQuantity.Float64(), 1.0)

		AssertOrdersPriceSideQuantity(t, []PriceSideQuantityAssert{
			{Side: types.SideTypeSell, Price: Number("2.0000"), Quantity: Number("151.34")},
			{Side: types.SideTypeSell, Price: Number("2.0013"), Quantity: Number("158.75")},
			{Side: types.SideTypeSell, Price: Number("2.0027"), Quantity: Number("166.52")},
			{Side: types.SideTypeSell, Price: Number("2.0041"), Quantity: Number("174.67")},
			{Side: types.SideTypeSell, Price: Number("2.0055"), Quantity: Number("183.23")},
			{Side: types.SideTypeSell, Price: Number("2.0068"), Quantity: Number("192.20")},
			{Side: types.SideTypeSell, Price: Number("2.0082"), Quantity: Number("201.61")},
			{Side: types.SideTypeSell, Price: Number("2.0096"), Quantity: Number("211.48")},
			{Side: types.SideTypeSell, Price: Number("2.0110"), Quantity: Number("221.84")},
			{Side: types.SideTypeSell, Price: Number("2.0124"), Quantity: Number("232.70")},
			{Side: types.SideTypeSell, Price: Number("2.0137"), Quantity: Number("244.09")},
			{Side: types.SideTypeSell, Price: Number("2.0151"), Quantity: Number("256.04")},
			{Side: types.SideTypeSell, Price: Number("2.0165"), Quantity: Number("268.58")},
			{Side: types.SideTypeSell, Price: Number("2.0179"), Quantity: Number("281.73")},
			{Side: types.SideTypeSell, Price: Number("2.0193"), Quantity: Number("295.53")},
		}, orders[0:15])

		AssertOrdersPriceSideQuantity(t, []PriceSideQuantityAssert{
			{Side: types.SideTypeSell, Price: Number("2.0386"), Quantity: Number("577.10")},
			{Side: types.SideTypeSell, Price: Number("2.0399"), Quantity: Number("605.36")},
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
			{Side: types.SideTypeBuy, Price: Number("2.0000"), Quantity: Number("155.13")},
			{Side: types.SideTypeBuy, Price: Number("1.9986"), Quantity: Number("162.73")},
			{Side: types.SideTypeBuy, Price: Number("1.9972"), Quantity: Number("170.70")},
			{Side: types.SideTypeBuy, Price: Number("1.9958"), Quantity: Number("179.05")},
			{Side: types.SideTypeBuy, Price: Number("1.9944"), Quantity: Number("187.82")},
			{Side: types.SideTypeBuy, Price: Number("1.9931"), Quantity: Number("197.02")},
			{Side: types.SideTypeBuy, Price: Number("1.9917"), Quantity: Number("206.67")},
			{Side: types.SideTypeBuy, Price: Number("1.9903"), Quantity: Number("216.79")},
			{Side: types.SideTypeBuy, Price: Number("1.9889"), Quantity: Number("227.40")},
			{Side: types.SideTypeBuy, Price: Number("1.9875"), Quantity: Number("238.54")},
			{Side: types.SideTypeBuy, Price: Number("1.9862"), Quantity: Number("250.22")},
			{Side: types.SideTypeBuy, Price: Number("1.9848"), Quantity: Number("262.47")},
			{Side: types.SideTypeBuy, Price: Number("1.9834"), Quantity: Number("275.32")},
			{Side: types.SideTypeBuy, Price: Number("1.9820"), Quantity: Number("288.80")},
			{Side: types.SideTypeBuy, Price: Number("1.9806"), Quantity: Number("302.94")},
		}, orders[0:15])

		AssertOrdersPriceSideQuantity(t, []PriceSideQuantityAssert{
			{Side: types.SideTypeBuy, Price: Number("1.9613"), Quantity: Number("591.58")},
			{Side: types.SideTypeBuy, Price: Number("1.9600"), Quantity: Number("620.54")},
		}, orders[28:30])
	})
}
