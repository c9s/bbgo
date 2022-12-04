//go:build !dnum

package grid2

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestStrategy_checkRequiredInvestmentByQuantity(t *testing.T) {
	s := &Strategy{
		logger: logrus.NewEntry(logrus.New()),

		Market: types.Market{
			BaseCurrency:  "BTC",
			QuoteCurrency: "USDT",
		},
	}

	t.Run("quote to base balance conversion check", func(t *testing.T) {
		_, requiredQuote, err := s.checkRequiredInvestmentByQuantity(number(0.0), number(10_000.0), number(0.1), number(13_500.0), []Pin{
			Pin(number(10_000.0)), // 0.1 * 10_000 = 1000 USD (buy)
			Pin(number(11_000.0)), // 0.1 * 11_000 = 1100 USD (buy)
			Pin(number(12_000.0)), // 0.1 * 12_000 = 1200 USD (buy)
			Pin(number(13_000.0)), // 0.1 * 13_000 = 1300 USD (buy)
			Pin(number(14_000.0)), // 0.1 * 14_000 = 1400 USD (buy)
			Pin(number(15_000.0)), // 0.1 * 15_000 = 1500 USD
		})
		assert.NoError(t, err)
		assert.Equal(t, number(6000.0), requiredQuote)
	})

	t.Run("quote to base balance conversion not enough", func(t *testing.T) {
		_, requiredQuote, err := s.checkRequiredInvestmentByQuantity(number(0.0), number(5_000.0), number(0.1), number(13_500.0), []Pin{
			Pin(number(10_000.0)), // 0.1 * 10_000 = 1000 USD (buy)
			Pin(number(11_000.0)), // 0.1 * 11_000 = 1100 USD (buy)
			Pin(number(12_000.0)), // 0.1 * 12_000 = 1200 USD (buy)
			Pin(number(13_000.0)), // 0.1 * 13_000 = 1300 USD (buy)
			Pin(number(14_000.0)), // 0.1 * 14_000 = 1400 USD (buy)
			Pin(number(15_000.0)), // 0.1 * 15_000 = 1500 USD
		})
		assert.EqualError(t, err, "quote balance (5000.000000 USDT) is not enough, required = quote 6000.000000")
		assert.Equal(t, number(6000.0), requiredQuote)
	})
}

func TestStrategy_checkRequiredInvestmentByAmount(t *testing.T) {
	s := &Strategy{

		logger: logrus.NewEntry(logrus.New()),
		Market: types.Market{
			BaseCurrency:  "BTC",
			QuoteCurrency: "USDT",
		},
	}

	t.Run("quote to base balance conversion", func(t *testing.T) {
		_, requiredQuote, err := s.checkRequiredInvestmentByAmount(
			number(0.0), number(3_000.0),
			number(1000.0),
			number(13_500.0), []Pin{
				Pin(number(10_000.0)),
				Pin(number(11_000.0)),
				Pin(number(12_000.0)),
				Pin(number(13_000.0)),
				Pin(number(14_000.0)),
				Pin(number(15_000.0)),
			})
		assert.EqualError(t, err, "quote balance (3000.000000 USDT) is not enough, required = quote 4999.999890")
		assert.InDelta(t, 4999.99989, requiredQuote.Float64(), number(0.001).Float64())
	})
}

func TestStrategy_calculateQuoteInvestmentQuantity(t *testing.T) {
	s := &Strategy{
		logger: logrus.NewEntry(logrus.New()),
		Market: types.Market{
			BaseCurrency:  "BTC",
			QuoteCurrency: "USDT",
		},
	}

	t.Run("calculate quote quantity from quote investment", func(t *testing.T) {
		// quoteInvestment = (10,000 + 11,000 + 12,000 + 13,000 + 14,000) * q
		// q = quoteInvestment / (10,000 + 11,000 + 12,000 + 13,000 + 14,000)
		// q = 12_000 / (10,000 + 11,000 + 12,000 + 13,000 + 14,000)
		// q = 0.2
		quantity, err := s.calculateQuoteInvestmentQuantity(number(12_000.0), number(13_500.0), []Pin{
			Pin(number(10_000.0)), // buy
			Pin(number(11_000.0)), // buy
			Pin(number(12_000.0)), // buy
			Pin(number(13_000.0)), // buy
			Pin(number(14_000.0)), // buy
			Pin(number(15_000.0)),
		})
		assert.NoError(t, err)
		assert.Equal(t, number(0.2).String(), quantity.String())
	})
}

func newTestStrategy() *Strategy {
	market := types.Market{
		BaseCurrency:  "BTC",
		QuoteCurrency: "USDT",
	}

	s := &Strategy{
		logger:          logrus.NewEntry(logrus.New()),
		Market:          market,
		GridProfitStats: newGridProfitStats(market),
		UpperPrice:      number(20_000),
		LowerPrice:      number(10_000),
		GridNum:         10,
	}
	return s
}

func TestStrategy_calculateProfit(t *testing.T) {
	t.Run("earn quote without compound", func(t *testing.T) {
		s := newTestStrategy()
		profit := s.calculateProfit(types.Order{
			SubmitOrder: types.SubmitOrder{
				Price:    number(13_000),
				Quantity: number(1.0),
			},
		}, number(12_000), number(1.0))
		assert.NotNil(t, profit)
		assert.Equal(t, "USDT", profit.Currency)
		assert.InDelta(t, 1000.0, profit.Profit.Float64(), 0.1)
	})

	t.Run("earn quote with compound", func(t *testing.T) {
		s := newTestStrategy()
		s.Compound = true

		profit := s.calculateProfit(types.Order{
			SubmitOrder: types.SubmitOrder{
				Price:    number(13_000),
				Quantity: number(1.0),
			},
		}, number(12_000), number(1.0))
		assert.NotNil(t, profit)
		assert.Equal(t, "USDT", profit.Currency)
		assert.InDelta(t, 1000.0, profit.Profit.Float64(), 0.1)
	})

	t.Run("earn base without compound", func(t *testing.T) {
		s := newTestStrategy()
		s.EarnBase = true
		s.Compound = false

		quoteQuantity := number(12_000).Mul(number(1.0))
		sellQuantity := quoteQuantity.Div(number(13_000.0))

		buyOrder := types.SubmitOrder{
			Price:    number(12_000.0),
			Quantity: number(1.0),
		}

		profit := s.calculateProfit(types.Order{
			SubmitOrder: types.SubmitOrder{
				Price:    number(13_000.0),
				Quantity: sellQuantity,
			},
		}, buyOrder.Price, buyOrder.Quantity)
		assert.NotNil(t, profit)
		assert.Equal(t, "BTC", profit.Currency)
		assert.InDelta(t, sellQuantity.Float64()-buyOrder.Quantity.Float64(), profit.Profit.Float64(), 0.001)
	})
}
