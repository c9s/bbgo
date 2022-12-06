//go:build !dnum

package grid2

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
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

type PriceSideAssert struct {
	Price fixedpoint.Value
	Side  types.SideType
}

func assertPriceSide(t *testing.T, priceSideAsserts []PriceSideAssert, orders []types.SubmitOrder) {
	for i, a := range priceSideAsserts {
		assert.Equalf(t, a.Price, orders[i].Price, "order #%d price should be %f", i+1, a.Price.Float64())
		assert.Equalf(t, a.Side, orders[i].Side, "order at price %f should be %s", a.Price.Float64(), a.Side)
	}
}

func TestStrategy_generateGridOrders(t *testing.T) {
	t.Run("quote only", func(t *testing.T) {
		s := newTestStrategy()
		s.grid = NewGrid(s.LowerPrice, s.UpperPrice, fixedpoint.NewFromInt(s.GridNum), s.Market.TickSize)
		s.grid.CalculateArithmeticPins()
		s.QuantityOrAmount.Quantity = number(0.01)

		lastPrice := number(15300)
		orders, err := s.generateGridOrders(number(10000.0), number(0), lastPrice)
		assert.NoError(t, err)
		if !assert.Equal(t, 10, len(orders)) {
			for _, o := range orders {
				t.Logf("- %s %s", o.Price.String(), o.Side)
			}
		}

		assertPriceSide(t, []PriceSideAssert{
			{number(19000.0), types.SideTypeBuy},
			{number(18000.0), types.SideTypeBuy},
			{number(17000.0), types.SideTypeBuy},
			{number(16000.0), types.SideTypeBuy},
			{number(15000.0), types.SideTypeBuy},
			{number(14000.0), types.SideTypeBuy},
			{number(13000.0), types.SideTypeBuy},
			{number(12000.0), types.SideTypeBuy},
			{number(11000.0), types.SideTypeBuy},
			{number(10000.0), types.SideTypeBuy},
		}, orders)
	})

	t.Run("base + quote", func(t *testing.T) {
		s := newTestStrategy()
		s.grid = NewGrid(s.LowerPrice, s.UpperPrice, fixedpoint.NewFromInt(s.GridNum), s.Market.TickSize)
		s.grid.CalculateArithmeticPins()
		s.QuantityOrAmount.Quantity = number(0.01)

		lastPrice := number(15300)
		orders, err := s.generateGridOrders(number(10000.0), number(0.021), lastPrice)
		assert.NoError(t, err)
		if !assert.Equal(t, 10, len(orders)) {
			for _, o := range orders {
				t.Logf("- %s %s", o.Price.String(), o.Side)
			}
		}

		assertPriceSide(t, []PriceSideAssert{
			{number(20000.0), types.SideTypeSell},
			{number(19000.0), types.SideTypeSell},
			{number(17000.0), types.SideTypeBuy},
			{number(16000.0), types.SideTypeBuy},
			{number(15000.0), types.SideTypeBuy},
			{number(14000.0), types.SideTypeBuy},
			{number(13000.0), types.SideTypeBuy},
			{number(12000.0), types.SideTypeBuy},
			{number(11000.0), types.SideTypeBuy},
			{number(10000.0), types.SideTypeBuy},
		}, orders)
	})

	t.Run("enough base + quote", func(t *testing.T) {
		s := newTestStrategy()
		s.grid = NewGrid(s.LowerPrice, s.UpperPrice, fixedpoint.NewFromInt(s.GridNum), s.Market.TickSize)
		s.grid.CalculateArithmeticPins()
		s.QuantityOrAmount.Quantity = number(0.01)

		lastPrice := number(15300)
		orders, err := s.generateGridOrders(number(10000.0), number(1.0), lastPrice)
		assert.NoError(t, err)
		if !assert.Equal(t, 10, len(orders)) {
			for _, o := range orders {
				t.Logf("- %s %s", o.Price.String(), o.Side)
			}
		}

		assertPriceSide(t, []PriceSideAssert{
			{number(20000.0), types.SideTypeSell},
			{number(19000.0), types.SideTypeSell},
			{number(18000.0), types.SideTypeSell},
			{number(17000.0), types.SideTypeSell},
			{number(16000.0), types.SideTypeSell},
			{number(14000.0), types.SideTypeBuy},
			{number(13000.0), types.SideTypeBuy},
			{number(12000.0), types.SideTypeBuy},
			{number(11000.0), types.SideTypeBuy},
			{number(10000.0), types.SideTypeBuy},
		}, orders)
	})

	t.Run("enough base + quote + profitSpread", func(t *testing.T) {
		s := newTestStrategy()
		s.ProfitSpread = number(1_000)
		s.grid = NewGrid(s.LowerPrice, s.UpperPrice, fixedpoint.NewFromInt(s.GridNum), s.Market.TickSize)
		s.grid.CalculateArithmeticPins()
		s.QuantityOrAmount.Quantity = number(0.01)

		lastPrice := number(15300)
		orders, err := s.generateGridOrders(number(10000.0), number(1.0), lastPrice)
		assert.NoError(t, err)
		if !assert.Equal(t, 11, len(orders)) {
			for _, o := range orders {
				t.Logf("- %s %s", o.Price.String(), o.Side)
			}
		}

		assertPriceSide(t, []PriceSideAssert{
			{number(21000.0), types.SideTypeSell},
			{number(20000.0), types.SideTypeSell},
			{number(19000.0), types.SideTypeSell},
			{number(18000.0), types.SideTypeSell},
			{number(17000.0), types.SideTypeSell},

			{number(15000.0), types.SideTypeBuy},
			{number(14000.0), types.SideTypeBuy},
			{number(13000.0), types.SideTypeBuy},
			{number(12000.0), types.SideTypeBuy},
			{number(11000.0), types.SideTypeBuy},
			{number(10000.0), types.SideTypeBuy},
		}, orders)
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
		assert.InDelta(t, 4999.999890, requiredQuote.Float64(), number(0.001).Float64())
	})
}

func TestStrategy_calculateQuoteInvestmentQuantity(t *testing.T) {
	t.Run("quote quantity", func(t *testing.T) {
		// quoteInvestment = (10,000 + 11,000 + 12,000 + 13,000 + 14,000) * q
		// q = quoteInvestment / (10,000 + 11,000 + 12,000 + 13,000 + 14,000)
		// q = 12_000 / (10,000 + 11,000 + 12,000 + 13,000 + 14,000)
		// q = 0.2
		s := newTestStrategy()
		lastPrice := number(13_500.0)
		quantity, err := s.calculateQuoteInvestmentQuantity(number(12_000.0), lastPrice, []Pin{
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

	t.Run("profit spread", func(t *testing.T) {
		// quoteInvestment = (10,000 + 11,000 + 12,000 + 13,000 + 14,000 + 15,000) * q
		// q = quoteInvestment / (10,000 + 11,000 + 12,000 + 13,000 + 14,000 + 15,000)
		// q = 7500 / (10,000 + 11,000 + 12,000 + 13,000 + 14,000 + 15,000)
		// q = 0.1
		s := newTestStrategy()
		s.ProfitSpread = number(2000.0)
		lastPrice := number(13_500.0)
		quantity, err := s.calculateQuoteInvestmentQuantity(number(7500.0), lastPrice, []Pin{
			Pin(number(10_000.0)), // sell order @ 12_000
			Pin(number(11_000.0)), // sell order @ 13_000
			Pin(number(12_000.0)), // sell order @ 14_000
			Pin(number(13_000.0)), // sell order @ 15_000
			Pin(number(14_000.0)), // sell order @ 16_000
			Pin(number(15_000.0)), // sell order @ 17_000
		})
		assert.NoError(t, err)
		assert.Equal(t, number(0.1).String(), quantity.String())
	})

}

func newTestStrategy() *Strategy {
	market := types.Market{
		BaseCurrency:    "BTC",
		QuoteCurrency:   "USDT",
		TickSize:        number(0.01),
		PricePrecision:  2,
		VolumePrecision: 8,
		MinNotional:     number(10.0),
		MinQuantity:     number(0.001),
	}

	s := &Strategy{
		logger:           logrus.NewEntry(logrus.New()),
		Market:           market,
		GridProfitStats:  newGridProfitStats(market),
		UpperPrice:       number(20_000),
		LowerPrice:       number(10_000),
		GridNum:          10,
		historicalTrades: bbgo.NewTradeStore(),

		// QuoteInvestment: number(9000.0),
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

func TestStrategy_aggregateOrderBaseFee(t *testing.T) {
	s := newTestStrategy()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockService := mocks.NewMockExchangeOrderQueryService(mockCtrl)
	s.orderQueryService = mockService

	ctx := context.Background()
	mockService.EXPECT().QueryOrderTrades(ctx, types.OrderQuery{
		Symbol:  "BTCUSDT",
		OrderID: "3",
	}).Return([]types.Trade{
		{
			ID:          1,
			OrderID:     3,
			Exchange:    "binance",
			Price:       number(20000.0),
			Quantity:    number(0.2),
			Symbol:      "BTCUSDT",
			Side:        types.SideTypeBuy,
			IsBuyer:     true,
			FeeCurrency: "BTC",
			Fee:         number(0.2 * 0.01),
		},
		{
			ID:          1,
			OrderID:     3,
			Exchange:    "binance",
			Price:       number(20000.0),
			Quantity:    number(0.8),
			Symbol:      "BTCUSDT",
			Side:        types.SideTypeBuy,
			IsBuyer:     true,
			FeeCurrency: "BTC",
			Fee:         number(0.8 * 0.01),
		},
	}, nil)

	baseFee := s.aggregateOrderBaseFee(types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:       "BTCUSDT",
			Side:         types.SideTypeBuy,
			Type:         types.OrderTypeLimit,
			Quantity:     number(1.0),
			Price:        number(20000.0),
			AveragePrice: number(0),
			StopPrice:    number(0),
			Market:       types.Market{},
			TimeInForce:  types.TimeInForceGTC,
		},
		Exchange:         "binance",
		GID:              1,
		OrderID:          3,
		Status:           types.OrderStatusFilled,
		ExecutedQuantity: number(1.0),
		IsWorking:        false,
	})
	assert.Equal(t, "0.01", baseFee.String())
}

func TestStrategy_aggregateOrderBaseFeeRetry(t *testing.T) {
	s := newTestStrategy()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockService := mocks.NewMockExchangeOrderQueryService(mockCtrl)
	s.orderQueryService = mockService

	ctx := context.Background()
	mockService.EXPECT().QueryOrderTrades(ctx, types.OrderQuery{
		Symbol:  "BTCUSDT",
		OrderID: "3",
	}).Return(nil, errors.New("api error"))

	mockService.EXPECT().QueryOrderTrades(ctx, types.OrderQuery{
		Symbol:  "BTCUSDT",
		OrderID: "3",
	}).Return([]types.Trade{
		{
			ID:          1,
			OrderID:     3,
			Exchange:    "binance",
			Price:       number(20000.0),
			Quantity:    number(0.2),
			Symbol:      "BTCUSDT",
			Side:        types.SideTypeBuy,
			IsBuyer:     true,
			FeeCurrency: "BTC",
			Fee:         number(0.2 * 0.01),
		},
		{
			ID:          1,
			OrderID:     3,
			Exchange:    "binance",
			Price:       number(20000.0),
			Quantity:    number(0.8),
			Symbol:      "BTCUSDT",
			Side:        types.SideTypeBuy,
			IsBuyer:     true,
			FeeCurrency: "BTC",
			Fee:         number(0.8 * 0.01),
		},
	}, nil)

	baseFee := s.aggregateOrderBaseFee(types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:       "BTCUSDT",
			Side:         types.SideTypeBuy,
			Type:         types.OrderTypeLimit,
			Quantity:     number(1.0),
			Price:        number(20000.0),
			AveragePrice: number(0),
			StopPrice:    number(0),
			Market:       types.Market{},
			TimeInForce:  types.TimeInForceGTC,
		},
		Exchange:         "binance",
		GID:              1,
		OrderID:          3,
		Status:           types.OrderStatusFilled,
		ExecutedQuantity: number(1.0),
		IsWorking:        false,
	})
	assert.Equal(t, "0.01", baseFee.String())
}

func TestStrategy_checkMinimalQuoteInvestment(t *testing.T) {
	s := newTestStrategy()

	t.Run("10 grids", func(t *testing.T) {
		s.QuoteInvestment = number(10_000)
		s.GridNum = 10
		minQuoteInvestment := calculateMinimalQuoteInvestment(s.Market, s.LowerPrice, s.UpperPrice, s.GridNum)
		assert.Equal(t, "200", minQuoteInvestment.String())

		err := s.checkMinimalQuoteInvestment()
		assert.NoError(t, err)
	})

	t.Run("1000 grids", func(t *testing.T) {
		s.QuoteInvestment = number(10_000)
		s.GridNum = 1000
		minQuoteInvestment := calculateMinimalQuoteInvestment(s.Market, s.LowerPrice, s.UpperPrice, s.GridNum)
		assert.Equal(t, "20000", minQuoteInvestment.String())

		err := s.checkMinimalQuoteInvestment()
		assert.Error(t, err)
		assert.EqualError(t, err, "need at least 20000.000000 USDT for quote investment, 10000.000000 USDT given")
	})
}

func TestBacktestStrategy(t *testing.T) {
	market := types.Market{
		BaseCurrency:    "BTC",
		QuoteCurrency:   "USDT",
		TickSize:        number(0.01),
		PricePrecision:  2,
		VolumePrecision: 8,
	}
	strategy := &Strategy{
		logger:          logrus.NewEntry(logrus.New()),
		Symbol:          "BTCUSDT",
		Market:          market,
		GridProfitStats: newGridProfitStats(market),
		UpperPrice:      number(60_000),
		LowerPrice:      number(28_000),
		GridNum:         100,
		QuoteInvestment: number(9000.0),
	}
	RunBacktest(t, strategy)
}
