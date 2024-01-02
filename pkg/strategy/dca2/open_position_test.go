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
		Symbol:          "BTCUSDT",
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
		logger:          logrus.NewEntry(logrus.New()),
		Symbol:          symbol,
		Market:          market,
		TakeProfitRatio: Number("10%"),
	}
	return s
}

func TestGenerateOpenPositionOrders(t *testing.T) {
	assert := assert.New(t)

	strategy := newTestStrategy()

	t.Run("case 1: all config is valid and we can place enough orders", func(t *testing.T) {
		quoteInvestment := Number("10500")
		askPrice := Number("30000")
		margin := Number("0.05")
		submitOrders, err := generateOpenPositionOrders(strategy.Market, quoteInvestment, askPrice, margin, 4, strategy.OrderGroupID)
		if !assert.NoError(err) {
			return
		}

		assert.Len(submitOrders, 4)
		assert.Equal(Number("30000"), submitOrders[0].Price)
		assert.Equal(Number("0.0875"), submitOrders[0].Quantity)
		assert.Equal(Number("28500"), submitOrders[1].Price)
		assert.Equal(Number("0.092105"), submitOrders[1].Quantity)
		assert.Equal(Number("27075"), submitOrders[2].Price)
		assert.Equal(Number("0.096952"), submitOrders[2].Quantity)
		assert.Equal(Number("25721.25"), submitOrders[3].Price)
		assert.Equal(Number("0.102055"), submitOrders[3].Quantity)
	})

	t.Run("case 2: some orders' price will below 0, so we should not create such order", func(t *testing.T) {
	})

	t.Run("case 3: notional is too small, so we should decrease num of orders", func(t *testing.T) {
	})

	t.Run("case 4: quantity is too small, so we should decrease num of orders", func(t *testing.T) {
	})
}
