package dca2

import (
	"testing"

	"github.com/sirupsen/logrus"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func newTestMarket() types.Market {
	return types.Market{
		BaseCurrency:    "BTC",
		QuoteCurrency:   "USDT",
		TickSize:        Number(0.01),
		StepSize:        Number(0.000001),
		PricePrecision:  2,
		VolumePrecision: 8,
		MinNotional:     Number(8.0),
		MinQuantity:     Number(0.0003),
	}
}

func newTestStrategy(va ...string) *Strategy {
	symbol := "BTCUSDT"

	if len(va) > 0 {
		symbol = va[0]
	}

	market := newTestMarket()
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

	t.Run("case 1: all config is valid and we can place enough orders", func(t *testing.T) {
		budget := Number("105000")
		askPrice := Number("30000")
		margin := Number("0.05")
		submitOrders, err := strategy.generateMakerOrder(false, budget, askPrice, margin, 4)
		if !assert.NoError(err) {
			return
		}

		assert.Len(submitOrders, 4)
		assert.Equal(submitOrders[0].Price, Number("28500"))
		assert.Equal(submitOrders[1].Price, Number("27000"))
		assert.Equal(submitOrders[2].Price, Number("25500"))
		assert.Equal(submitOrders[3].Price, Number("24000"))
		assert.Equal(submitOrders[0].Quantity, Number("1"))
		assert.Equal(submitOrders[1].Quantity, Number("1"))
		assert.Equal(submitOrders[2].Quantity, Number("1"))
		assert.Equal(submitOrders[3].Quantity, Number("1"))
	})

	t.Run("case 2: some orders' price will below 0 and we should not create such order", func(t *testing.T) {
		budget := Number("100000")
		askPrice := Number("30000")
		margin := Number("0.2")
		submitOrders, err := strategy.generateMakerOrder(false, budget, askPrice, margin, 5)
		if !assert.NoError(err) {
			return
		}

		assert.Len(submitOrders, 4)
		assert.Equal(submitOrders[0].Price, Number("24000"))
		assert.Equal(submitOrders[1].Price, Number("18000"))
		assert.Equal(submitOrders[2].Price, Number("12000"))
		assert.Equal(submitOrders[3].Price, Number("6000"))
		assert.Equal(submitOrders[0].Quantity, Number("1.666666"))
		assert.Equal(submitOrders[1].Quantity, Number("1.666666"))
		assert.Equal(submitOrders[2].Quantity, Number("1.666666"))
		assert.Equal(submitOrders[3].Quantity, Number("1.666666"))
	})

	t.Run("case 3: some orders' notional is too small and we should not create such order", func(t *testing.T) {
		budget := Number("30")
		askPrice := Number("30000")
		margin := Number("0.2")
		submitOrders, err := strategy.generateMakerOrder(false, budget, askPrice, margin, 5)
		if !assert.NoError(err) {
			return
		}

		assert.Len(submitOrders, 2)
		assert.Equal(submitOrders[0].Price, Number("24000"))
		assert.Equal(submitOrders[1].Price, Number("18000"))
		assert.Equal(submitOrders[0].Quantity, Number("0.000714"))
		assert.Equal(submitOrders[1].Quantity, Number("0.000714"))
	})
}
