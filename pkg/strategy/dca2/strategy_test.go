package dca2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func number(a interface{}) fixedpoint.Value {
	switch v := a.(type) {
	case string:
		return fixedpoint.MustNewFromString(v)
	case int:
		return fixedpoint.NewFromInt(int64(v))
	case int64:
		return fixedpoint.NewFromInt(int64(v))
	case float64:
		return fixedpoint.NewFromFloat(v)
	}

	return fixedpoint.Zero
}

func newTestMarket(symbol string) types.Market {
	switch symbol {
	case "BTCUSDT":
		return types.Market{
			BaseCurrency:    "BTC",
			QuoteCurrency:   "USDT",
			TickSize:        number(0.01),
			StepSize:        number(0.000001),
			PricePrecision:  2,
			VolumePrecision: 8,
			MinNotional:     number(8.0),
			MinQuantity:     number(0.0003),
		}
	case "ETHUSDT":
		return types.Market{
			BaseCurrency:    "ETH",
			QuoteCurrency:   "USDT",
			TickSize:        number(0.01),
			StepSize:        number(0.00001),
			PricePrecision:  2,
			VolumePrecision: 6,
			MinNotional:     number(8.000),
			MinQuantity:     number(0.0046),
		}
	}

	// default
	return types.Market{
		BaseCurrency:    "BTC",
		QuoteCurrency:   "USDT",
		TickSize:        number(0.01),
		StepSize:        number(0.00001),
		PricePrecision:  2,
		VolumePrecision: 8,
		MinNotional:     number(10.0),
		MinQuantity:     number(0.001),
	}
}

func newTestStrategy(va ...string) *Strategy {
	symbol := "BTCUSDT"

	if len(va) > 0 {
		symbol = va[0]
	}

	market := newTestMarket(symbol)
	s := &Strategy{
		logger: logrus.NewEntry(logrus.New()),
		Symbol: symbol,
		Market: market,
	}
	return s
}

func TestGenerateMakerOrder(t *testing.T) {
	assert := assert.New(t)

	strategy := newTestStrategy()

	budget := number("105000")
	askPrice := number("30000")
	margin := number("0.05")
	submitOrders, err := strategy.generateMakerOrder(budget, askPrice, margin, 4)
	if !assert.NoError(err) {
		return
	}

	assert.Len(submitOrders, 4)
	assert.Equal(submitOrders[0].Price, number("28500"))
	assert.Equal(submitOrders[0].Quantity, number("1"))
	assert.Equal(submitOrders[1].Price, number("27000"))
	assert.Equal(submitOrders[1].Quantity, number("1"))
	assert.Equal(submitOrders[2].Price, number("25500"))
	assert.Equal(submitOrders[2].Quantity, number("1"))
	assert.Equal(submitOrders[3].Price, number("24000"))
	assert.Equal(submitOrders[3].Quantity, number("1"))
}

func TestGenerateTakeProfitOrder(t *testing.T) {
	assert := assert.New(t)

	strategy := newTestStrategy()

	position := types.NewPositionFromMarket(strategy.Market)
	position.AddTrade(types.Trade{
		Side:          types.SideTypeBuy,
		Price:         number("28500"),
		Quantity:      number("1"),
		QuoteQuantity: number("28500"),
		Fee:           number("0.0015"),
		FeeCurrency:   strategy.Market.BaseCurrency,
	})

	o := strategy.generateTakeProfitOrder(position, number("10%"))
	assert.Equal(number("31397.09"), o.Price)
	assert.Equal(number("0.9985"), o.Quantity)
	assert.Equal(types.SideTypeSell, o.Side)
	assert.Equal(strategy.Symbol, o.Symbol)

	position.AddTrade(types.Trade{
		Side:          types.SideTypeBuy,
		Price:         number("27000"),
		Quantity:      number("0.5"),
		QuoteQuantity: number("13500"),
		Fee:           number("0.00075"),
		FeeCurrency:   strategy.Market.BaseCurrency,
	})
	o = strategy.generateTakeProfitOrder(position, number("10%"))
	assert.Equal(number("30846.26"), o.Price)
	assert.Equal(number("1.49775"), o.Quantity)
	assert.Equal(types.SideTypeSell, o.Side)
	assert.Equal(strategy.Symbol, o.Symbol)

}
