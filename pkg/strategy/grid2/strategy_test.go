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
	gridmocks "github.com/c9s/bbgo/pkg/strategy/grid2/mocks"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

func init() {
	registerMetrics()
}

func equalOrdersIgnoreClientOrderID(a, b types.SubmitOrder) bool {
	return a.Symbol == b.Symbol &&
		a.Side == b.Side &&
		a.Type == b.Type &&
		a.Quantity == b.Quantity &&
		a.Price == b.Price &&
		a.AveragePrice == b.AveragePrice &&
		a.StopPrice == b.StopPrice &&
		a.Market == b.Market &&
		a.TimeInForce == b.TimeInForce &&
		a.GroupID == b.GroupID &&
		a.MarginSideEffect == b.MarginSideEffect &&
		a.ReduceOnly == b.ReduceOnly &&
		a.ClosePosition == b.ClosePosition &&
		a.Tag == b.Tag
}

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
		quoteInvestment := number(10000.0)
		baseInvestment := number(0)
		orders, err := s.generateGridOrders(quoteInvestment, baseInvestment, lastPrice)
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

	t.Run("quote only + buy only", func(t *testing.T) {
		s := newTestStrategy()
		s.UpperPrice = number(0.9)
		s.LowerPrice = number(0.1)
		s.GridNum = 7
		s.grid = NewGrid(s.LowerPrice, s.UpperPrice, fixedpoint.NewFromInt(s.GridNum), s.Market.TickSize)
		s.grid.CalculateArithmeticPins()

		assert.Equal(t, []Pin{
			Pin(number(0.1)),
			Pin(number(0.23)),
			Pin(number(0.36)),
			Pin(number(0.50)),
			Pin(number(0.63)),
			Pin(number(0.76)),
			Pin(number(0.9)),
		}, s.grid.Pins, "pins are correct")

		lastPrice := number(22100)
		quoteInvestment := number(100.0)
		baseInvestment := number(0)

		quantity, err := s.calculateQuoteInvestmentQuantity(quoteInvestment, lastPrice, s.grid.Pins)
		assert.NoError(t, err)
		assert.InDelta(t, 38.7364341, quantity.Float64(), 0.00001)

		s.QuantityOrAmount.Quantity = quantity

		orders, err := s.generateGridOrders(quoteInvestment, baseInvestment, lastPrice)
		assert.NoError(t, err)
		if !assert.Equal(t, 6, len(orders)) {
			for _, o := range orders {
				t.Logf("- %s %s", o.Price.String(), o.Side)
			}
		}

		assertPriceSide(t, []PriceSideAssert{
			{number(0.76), types.SideTypeBuy},
			{number(0.63), types.SideTypeBuy},
			{number(0.5), types.SideTypeBuy},
			{number(0.36), types.SideTypeBuy},
			{number(0.23), types.SideTypeBuy},
			{number(0.1), types.SideTypeBuy},
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
		quoteInvestment := number(12_000.0)
		quantity, err := s.calculateQuoteInvestmentQuantity(quoteInvestment, lastPrice, []Pin{
			Pin(number(10_000.0)), // buy
			Pin(number(11_000.0)), // buy
			Pin(number(12_000.0)), // buy
			Pin(number(13_000.0)), // buy
			Pin(number(14_000.0)), // buy
			Pin(number(15_000.0)),
		})
		assert.NoError(t, err)
		assert.InDelta(t, 0.199999916, quantity.Float64(), 0.0001)
	})

	t.Run("quote quantity #2", func(t *testing.T) {
		s := newTestStrategy()
		lastPrice := number(160.0)
		quoteInvestment := number(1_000.0)
		quantity, err := s.calculateQuoteInvestmentQuantity(quoteInvestment, lastPrice, []Pin{
			Pin(number(100.0)),  // buy
			Pin(number(116.67)), // buy
			Pin(number(133.33)), // buy
			Pin(number(150.00)), // buy
			Pin(number(166.67)), // buy
			Pin(number(183.33)),
			Pin(number(200.00)),
		})
		assert.NoError(t, err)
		assert.InDelta(t, 1.1764, quantity.Float64(), 0.00001)
	})

	t.Run("quote quantity #3", func(t *testing.T) {
		s := newTestStrategy()
		lastPrice := number(22000.0)
		quoteInvestment := number(100.0)
		pins := []Pin{
			Pin(number(0.1)),
			Pin(number(0.23)),
			Pin(number(0.36)),
			Pin(number(0.50)),
			Pin(number(0.63)),
			Pin(number(0.76)),
			Pin(number(0.90)),
		}
		quantity, err := s.calculateQuoteInvestmentQuantity(quoteInvestment, lastPrice, pins)
		assert.NoError(t, err)
		assert.InDelta(t, 38.736434, quantity.Float64(), 0.0001)

		var totalQuoteUsed = fixedpoint.Zero
		for i, pin := range pins {
			if i == len(pins)-1 {
				continue
			}

			price := fixedpoint.Value(pin)
			totalQuoteUsed = totalQuoteUsed.Add(price.Mul(quantity))
		}
		assert.LessOrEqualf(t, totalQuoteUsed, number(100.0), "total quote used: %f", totalQuoteUsed.Float64())
	})

	t.Run("profit spread", func(t *testing.T) {
		// quoteInvestment = (10,000 + 11,000 + 12,000 + 13,000 + 14,000 + 15,000) * q
		// q = quoteInvestment / (10,000 + 11,000 + 12,000 + 13,000 + 14,000 + 15,000)
		// q = 7500 / (10,000 + 11,000 + 12,000 + 13,000 + 14,000 + 15,000)
		// q = 0.1
		s := newTestStrategy()
		s.ProfitSpread = number(2000.0)
		lastPrice := number(13_500.0)
		quoteInvestment := number(7500.0)
		quantity, err := s.calculateQuoteInvestmentQuantity(quoteInvestment, lastPrice, []Pin{
			Pin(number(10_000.0)), // sell order @ 12_000
			Pin(number(11_000.0)), // sell order @ 13_000
			Pin(number(12_000.0)), // sell order @ 14_000
			Pin(number(13_000.0)), // sell order @ 15_000
			Pin(number(14_000.0)), // sell order @ 16_000
			Pin(number(15_000.0)), // sell order @ 17_000
		})
		assert.NoError(t, err)
		assert.InDelta(t, 0.099992, quantity.Float64(), 0.0001)
	})
}

func newTestMarket() types.Market {
	return types.Market{
		BaseCurrency:    "BTC",
		QuoteCurrency:   "USDT",
		TickSize:        number(0.01),
		PricePrecision:  2,
		VolumePrecision: 8,
		MinNotional:     number(10.0),
		MinQuantity:     number(0.001),
	}
}

var testOrderID = uint64(0)

func newTestOrder(price, quantity fixedpoint.Value, side types.SideType) types.Order {
	market := newTestMarket()
	testOrderID++
	return types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:       "BTCUSDT",
			Side:         side,
			Type:         types.OrderTypeLimit,
			Quantity:     quantity,
			Price:        price,
			AveragePrice: fixedpoint.Zero,
			StopPrice:    fixedpoint.Zero,
			Market:       market,
			TimeInForce:  types.TimeInForceGTC,
		},
		Exchange:         "binance",
		GID:              testOrderID,
		OrderID:          testOrderID,
		Status:           types.OrderStatusNew,
		ExecutedQuantity: fixedpoint.Zero,
		IsWorking:        true,
	}
}

func newTestStrategy() *Strategy {
	market := newTestMarket()

	s := &Strategy{
		logger:           logrus.NewEntry(logrus.New()),
		Symbol:           "BTCUSDT",
		Market:           market,
		GridProfitStats:  newGridProfitStats(market),
		UpperPrice:       number(20_000),
		LowerPrice:       number(10_000),
		GridNum:          11,
		historicalTrades: bbgo.NewTradeStore(),

		filledOrderIDMap: types.NewSyncOrderMap(),

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

	baseFee, _ := s.aggregateOrderFee(types.Order{
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

func TestStrategy_findDuplicatedPriceOpenOrders(t *testing.T) {
	t.Run("no duplicated open orders", func(t *testing.T) {
		s := newTestStrategy()
		s.grid = s.newGrid()

		dupOrders := s.findDuplicatedPriceOpenOrders([]types.Order{
			newTestOrder(number(1900.0), number(0.1), types.SideTypeSell),
			newTestOrder(number(1800.0), number(0.1), types.SideTypeSell),
			newTestOrder(number(1700.0), number(0.1), types.SideTypeSell),
		})
		assert.Empty(t, dupOrders)
		assert.Len(t, dupOrders, 0)
	})

	t.Run("1 duplicated open order SELL", func(t *testing.T) {
		s := newTestStrategy()
		s.grid = s.newGrid()

		dupOrders := s.findDuplicatedPriceOpenOrders([]types.Order{
			newTestOrder(number(1900.0), number(0.1), types.SideTypeSell),
			newTestOrder(number(1900.0), number(0.1), types.SideTypeSell),
			newTestOrder(number(1800.0), number(0.1), types.SideTypeSell),
		})
		assert.Len(t, dupOrders, 1)
	})
}

func TestStrategy_handleOrderFilled(t *testing.T) {
	ctx := context.Background()

	t.Run("no fee token", func(t *testing.T) {
		gridQuantity := number(0.1)
		orderID := uint64(1)

		s := newTestStrategy()
		s.Quantity = gridQuantity
		s.grid = s.newGrid()

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockService := mocks.NewMockExchangeOrderQueryService(mockCtrl)
		mockService.EXPECT().QueryOrderTrades(ctx, types.OrderQuery{
			Symbol:  "BTCUSDT",
			OrderID: "1",
		}).Return([]types.Trade{
			{
				ID:          1,
				OrderID:     orderID,
				Exchange:    "binance",
				Price:       number(11000.0),
				Quantity:    gridQuantity,
				Symbol:      "BTCUSDT",
				Side:        types.SideTypeBuy,
				IsBuyer:     true,
				FeeCurrency: "BTC",
				Fee:         number(gridQuantity.Float64() * 0.1 * 0.01),
			},
		}, nil)

		s.orderQueryService = mockService

		expectedSubmitOrder := types.SubmitOrder{
			Symbol:      "BTCUSDT",
			Type:        types.OrderTypeLimit,
			Price:       number(12_000.0),
			Quantity:    number(0.0999),
			Side:        types.SideTypeSell,
			TimeInForce: types.TimeInForceGTC,
			Market:      s.Market,
			Tag:         orderTag,
		}

		orderExecutor := gridmocks.NewMockOrderExecutor(mockCtrl)
		orderExecutor.EXPECT().SubmitOrders(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (types.OrderSlice, error) {
			assert.True(t, equalOrdersIgnoreClientOrderID(expectedSubmitOrder, order), "%+v is not equal to %+v", order, expectedSubmitOrder)
			return []types.Order{
				{SubmitOrder: expectedSubmitOrder},
			}, nil
		})
		s.orderExecutor = orderExecutor

		s.handleOrderFilled(types.Order{
			SubmitOrder: types.SubmitOrder{
				Symbol:      "BTCUSDT",
				Side:        types.SideTypeBuy,
				Type:        types.OrderTypeLimit,
				Quantity:    gridQuantity,
				Price:       number(11000.0),
				TimeInForce: types.TimeInForceGTC,
			},
			Exchange:         "binance",
			OrderID:          orderID,
			Status:           types.OrderStatusFilled,
			ExecutedQuantity: gridQuantity,
		})
	})

	t.Run("with fee token", func(t *testing.T) {
		gridQuantity := number(0.1)
		orderID := uint64(1)

		s := newTestStrategy()
		s.Quantity = gridQuantity
		s.grid = s.newGrid()

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockService := mocks.NewMockExchangeOrderQueryService(mockCtrl)
		mockService.EXPECT().QueryOrderTrades(ctx, types.OrderQuery{
			Symbol:  "BTCUSDT",
			OrderID: "1",
		}).Return([]types.Trade{
			{
				ID:          1,
				OrderID:     orderID,
				Exchange:    "binance",
				Price:       number(11000.0),
				Quantity:    gridQuantity,
				Symbol:      "BTCUSDT",
				Side:        types.SideTypeBuy,
				IsBuyer:     true,
				FeeCurrency: "BTC",
				Fee:         fixedpoint.Zero,
			},
		}, nil)

		s.orderQueryService = mockService

		expectedSubmitOrder := types.SubmitOrder{
			Symbol:      "BTCUSDT",
			Type:        types.OrderTypeLimit,
			Price:       number(12_000.0),
			Quantity:    gridQuantity,
			Side:        types.SideTypeSell,
			TimeInForce: types.TimeInForceGTC,
			Market:      s.Market,
			Tag:         orderTag,
		}

		orderExecutor := gridmocks.NewMockOrderExecutor(mockCtrl)
		orderExecutor.EXPECT().SubmitOrders(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (types.OrderSlice, error) {
			assert.True(t, equalOrdersIgnoreClientOrderID(expectedSubmitOrder, order), "%+v is not equal to %+v", order, expectedSubmitOrder)
			return []types.Order{
				{SubmitOrder: expectedSubmitOrder},
			}, nil
		})
		s.orderExecutor = orderExecutor

		s.handleOrderFilled(types.Order{
			SubmitOrder: types.SubmitOrder{
				Symbol:      "BTCUSDT",
				Side:        types.SideTypeBuy,
				Type:        types.OrderTypeLimit,
				Quantity:    gridQuantity,
				Price:       number(11000.0),
				TimeInForce: types.TimeInForceGTC,
			},
			Exchange:         "binance",
			OrderID:          orderID,
			Status:           types.OrderStatusFilled,
			ExecutedQuantity: gridQuantity,
		})
	})

	t.Run("with fee token and EarnBase", func(t *testing.T) {
		gridQuantity := number(0.1)
		orderID := uint64(1)

		s := newTestStrategy()
		s.Quantity = gridQuantity
		s.EarnBase = true
		s.grid = s.newGrid()

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockService := mocks.NewMockExchangeOrderQueryService(mockCtrl)

		mockService.EXPECT().QueryOrderTrades(ctx, types.OrderQuery{
			Symbol:  "BTCUSDT",
			OrderID: "1",
		}).Return([]types.Trade{
			{
				ID:          1,
				OrderID:     orderID,
				Exchange:    "binance",
				Price:       number(11000.0),
				Quantity:    number("0.1"),
				Symbol:      "BTCUSDT",
				Side:        types.SideTypeBuy,
				IsBuyer:     true,
				FeeCurrency: "BTC",
				Fee:         fixedpoint.Zero,
			},
		}, nil).Times(1)

		mockService.EXPECT().QueryOrderTrades(ctx, types.OrderQuery{
			Symbol:  "BTCUSDT",
			OrderID: "2",
		}).Return([]types.Trade{
			{
				ID:          2,
				OrderID:     orderID,
				Exchange:    "binance",
				Price:       number(12000.0),
				Quantity:    number(0.09166666666),
				Symbol:      "BTCUSDT",
				Side:        types.SideTypeSell,
				IsBuyer:     true,
				FeeCurrency: "BTC",
				Fee:         fixedpoint.Zero,
			},
		}, nil).Times(1)

		s.orderQueryService = mockService

		orderExecutor := gridmocks.NewMockOrderExecutor(mockCtrl)

		expectedSubmitOrder := types.SubmitOrder{
			Symbol:      "BTCUSDT",
			Type:        types.OrderTypeLimit,
			Side:        types.SideTypeSell,
			Price:       number(12_000.0),
			Quantity:    number(0.09166666),
			TimeInForce: types.TimeInForceGTC,
			Market:      s.Market,
			Tag:         orderTag,
		}
		orderExecutor.EXPECT().SubmitOrders(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (types.OrderSlice, error) {
			assert.True(t, equalOrdersIgnoreClientOrderID(expectedSubmitOrder, order), "%+v is not equal to %+v", order, expectedSubmitOrder)
			return []types.Order{
				{SubmitOrder: expectedSubmitOrder},
			}, nil
		})

		expectedSubmitOrder2 := types.SubmitOrder{
			Symbol:      "BTCUSDT",
			Type:        types.OrderTypeLimit,
			Side:        types.SideTypeBuy,
			Price:       number(11_000.0),
			Quantity:    number(0.09999909),
			TimeInForce: types.TimeInForceGTC,
			Market:      s.Market,
			Tag:         orderTag,
		}
		orderExecutor.EXPECT().SubmitOrders(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (types.OrderSlice, error) {
			assert.True(t, equalOrdersIgnoreClientOrderID(expectedSubmitOrder2, order), "%+v is not equal to %+v", order, expectedSubmitOrder2)
			return []types.Order{
				{SubmitOrder: expectedSubmitOrder2},
			}, nil
		})

		s.orderExecutor = orderExecutor

		s.handleOrderFilled(types.Order{
			SubmitOrder: types.SubmitOrder{
				Symbol:      "BTCUSDT",
				Side:        types.SideTypeBuy,
				Type:        types.OrderTypeLimit,
				Quantity:    gridQuantity,
				Price:       number(11000.0),
				TimeInForce: types.TimeInForceGTC,
			},
			Exchange:         "binance",
			OrderID:          1,
			Status:           types.OrderStatusFilled,
			ExecutedQuantity: gridQuantity,
		})

		s.handleOrderFilled(types.Order{
			SubmitOrder:      expectedSubmitOrder,
			Exchange:         "binance",
			OrderID:          2,
			Status:           types.OrderStatusFilled,
			ExecutedQuantity: expectedSubmitOrder.Quantity,
		})
	})

	t.Run("with fee token and compound", func(t *testing.T) {
		gridQuantity := number(0.1)
		orderID := uint64(1)

		s := newTestStrategy()
		s.Quantity = gridQuantity
		s.Compound = true
		s.grid = s.newGrid()

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockService := mocks.NewMockExchangeOrderQueryService(mockCtrl)

		mockService.EXPECT().QueryOrderTrades(ctx, types.OrderQuery{
			Symbol:  "BTCUSDT",
			OrderID: "1",
		}).Return([]types.Trade{
			{
				ID:          1,
				OrderID:     orderID,
				Exchange:    "binance",
				Price:       number(11000.0),
				Quantity:    gridQuantity,
				Symbol:      "BTCUSDT",
				Side:        types.SideTypeBuy,
				IsBuyer:     true,
				FeeCurrency: "BTC",
				Fee:         number("0.00001"),
			},
		}, nil)

		mockService.EXPECT().QueryOrderTrades(ctx, types.OrderQuery{
			Symbol:  "BTCUSDT",
			OrderID: "2",
		}).Return([]types.Trade{
			{
				ID:          2,
				OrderID:     orderID,
				Exchange:    "binance",
				Price:       number(12000.0),
				Quantity:    gridQuantity,
				Symbol:      "BTCUSDT",
				Side:        types.SideTypeSell,
				IsBuyer:     true,
				FeeCurrency: "USDT",
				Fee:         number("0.01"),
			},
		}, nil)

		s.orderQueryService = mockService

		expectedSubmitOrder := types.SubmitOrder{
			Symbol:      "BTCUSDT",
			Type:        types.OrderTypeLimit,
			Price:       number(12_000.0),
			Quantity:    number(0.09999),
			Side:        types.SideTypeSell,
			TimeInForce: types.TimeInForceGTC,
			Market:      s.Market,
			Tag:         orderTag,
		}

		orderExecutor := gridmocks.NewMockOrderExecutor(mockCtrl)
		orderExecutor.EXPECT().SubmitOrders(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (types.OrderSlice, error) {
			assert.True(t, equalOrdersIgnoreClientOrderID(expectedSubmitOrder, order), "%+v is not equal to %+v", order, expectedSubmitOrder)
			return []types.Order{
				{SubmitOrder: expectedSubmitOrder},
			}, nil
		})

		expectedSubmitOrder2 := types.SubmitOrder{
			Symbol:      "BTCUSDT",
			Type:        types.OrderTypeLimit,
			Price:       number(11_000.0),
			Quantity:    number(0.10909),
			Side:        types.SideTypeBuy,
			TimeInForce: types.TimeInForceGTC,
			Market:      s.Market,
			Tag:         orderTag,
		}

		orderExecutor.EXPECT().SubmitOrders(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (types.OrderSlice, error) {
			assert.True(t, equalOrdersIgnoreClientOrderID(expectedSubmitOrder2, order), "%+v is not equal to %+v", order, expectedSubmitOrder2)
			return []types.Order{
				{SubmitOrder: expectedSubmitOrder2},
			}, nil
		})
		s.orderExecutor = orderExecutor

		s.handleOrderFilled(types.Order{
			SubmitOrder: types.SubmitOrder{
				Symbol:      "BTCUSDT",
				Side:        types.SideTypeBuy,
				Type:        types.OrderTypeLimit,
				Quantity:    gridQuantity,
				Price:       number(11000.0),
				TimeInForce: types.TimeInForceGTC,
			},
			Exchange:         "binance",
			OrderID:          1,
			Status:           types.OrderStatusFilled,
			ExecutedQuantity: gridQuantity,
		})

		s.handleOrderFilled(types.Order{
			SubmitOrder: types.SubmitOrder{
				Symbol:      "BTCUSDT",
				Side:        types.SideTypeSell,
				Type:        types.OrderTypeLimit,
				Quantity:    gridQuantity,
				Price:       number(12000.0),
				TimeInForce: types.TimeInForceGTC,
			},
			Exchange:         "binance",
			OrderID:          2,
			Status:           types.OrderStatusFilled,
			ExecutedQuantity: gridQuantity,
		})

	})
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

	baseFee, _ := s.aggregateOrderFee(types.Order{
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

	t.Run("7 grids", func(t *testing.T) {
		s := newTestStrategy()
		s.UpperPrice = number(1660)
		s.LowerPrice = number(1630)
		s.QuoteInvestment = number(61)
		s.GridNum = 7
		grid := s.newGrid()
		minQuoteInvestment := calculateMinimalQuoteInvestment(s.Market, grid)
		assert.InDelta(t, 60.46, minQuoteInvestment.Float64(), 0.01)

		err := s.checkMinimalQuoteInvestment(grid)
		assert.NoError(t, err)
	})

	t.Run("10 grids", func(t *testing.T) {
		s := newTestStrategy()
		// 10_000 * 0.001 = 10USDT
		// 20_000 * 0.001 = 20USDT
		// hence we should have at least: 20USDT * 10 grids
		s.QuoteInvestment = number(10_000)
		s.GridNum = 10
		grid := s.newGrid()
		minQuoteInvestment := calculateMinimalQuoteInvestment(s.Market, grid)
		assert.InDelta(t, 129.9999, minQuoteInvestment.Float64(), 0.01)

		err := s.checkMinimalQuoteInvestment(grid)
		assert.NoError(t, err)
	})

	t.Run("1000 grids", func(t *testing.T) {
		s := newTestStrategy()
		s.QuoteInvestment = number(10_000)
		s.GridNum = 1000

		grid := s.newGrid()
		minQuoteInvestment := calculateMinimalQuoteInvestment(s.Market, grid)
		assert.InDelta(t, 14979.995499, minQuoteInvestment.Float64(), 0.001)

		err := s.checkMinimalQuoteInvestment(grid)
		assert.Error(t, err)
		assert.EqualError(t, err, "need at least 14979.995500 USDT for quote investment, 10000.000000 USDT given")
	})
}

/*
func Test_buildPinOrderMap(t *testing.T) {
	assert := assert.New(t)
	s := newTestStrategy()
	s.UpperPrice = number(2000.0)
	s.LowerPrice = number(1000.0)
	s.GridNum = 11
	s.grid = s.newGrid()

	t.Run("successful case", func(t *testing.T) {
		openOrders := []types.Order{
			types.Order{
				SubmitOrder: types.SubmitOrder{
					Symbol:       s.Symbol,
					Side:         types.SideTypeBuy,
					Type:         types.OrderTypeLimit,
					Quantity:     number(1.0),
					Price:        number(1000.0),
					AveragePrice: number(0),
					StopPrice:    number(0),
					Market:       s.Market,
					TimeInForce:  types.TimeInForceGTC,
				},
				Exchange:         "max",
				GID:              1,
				OrderID:          1,
				Status:           types.OrderStatusNew,
				ExecutedQuantity: number(0.0),
				IsWorking:        false,
			},
		}
		m, err := s.buildPinOrderMap(s.grid.Pins, openOrders)
		assert.NoError(err)
		assert.Len(m, 11)

		for pin, order := range m {
			if pin == openOrders[0].Price {
				assert.Equal(openOrders[0].OrderID, order.OrderID)
			} else {
				assert.Equal(uint64(0), order.OrderID)
			}
		}
	})

	t.Run("there is one order with non-pin price in openOrders", func(t *testing.T) {
		openOrders := []types.Order{
			types.Order{
				SubmitOrder: types.SubmitOrder{
					Symbol:       s.Symbol,
					Side:         types.SideTypeBuy,
					Type:         types.OrderTypeLimit,
					Quantity:     number(1.0),
					Price:        number(1111.0),
					AveragePrice: number(0),
					StopPrice:    number(0),
					Market:       s.Market,
					TimeInForce:  types.TimeInForceGTC,
				},
				Exchange:         "max",
				GID:              1,
				OrderID:          1,
				Status:           types.OrderStatusNew,
				ExecutedQuantity: number(0.0),
				IsWorking:        false,
			},
		}
		_, err := s.buildPinOrderMap(s.grid.Pins, openOrders)
		assert.Error(err)
	})

	t.Run("there are duplicated open orders at same pin", func(t *testing.T) {
		openOrders := []types.Order{
			types.Order{
				SubmitOrder: types.SubmitOrder{
					Symbol:       s.Symbol,
					Side:         types.SideTypeBuy,
					Type:         types.OrderTypeLimit,
					Quantity:     number(1.0),
					Price:        number(1000.0),
					AveragePrice: number(0),
					StopPrice:    number(0),
					Market:       s.Market,
					TimeInForce:  types.TimeInForceGTC,
				},
				Exchange:         "max",
				GID:              1,
				OrderID:          1,
				Status:           types.OrderStatusNew,
				ExecutedQuantity: number(0.0),
				IsWorking:        false,
			},
			types.Order{
				SubmitOrder: types.SubmitOrder{
					Symbol:       s.Symbol,
					Side:         types.SideTypeBuy,
					Type:         types.OrderTypeLimit,
					Quantity:     number(1.0),
					Price:        number(1000.0),
					AveragePrice: number(0),
					StopPrice:    number(0),
					Market:       s.Market,
					TimeInForce:  types.TimeInForceGTC,
				},
				Exchange:         "max",
				GID:              2,
				OrderID:          2,
				Status:           types.OrderStatusNew,
				ExecutedQuantity: number(0.0),
				IsWorking:        false,
			},
		}
		_, err := s.buildPinOrderMap(s.grid.Pins, openOrders)
		assert.Error(err)
	})
}

func Test_getOrdersFromPinOrderMapInAscOrder(t *testing.T) {
	assert := assert.New(t)
	now := time.Now()
	pinOrderMap := PinOrderMap{
		number("1000"): types.Order{
			OrderID:      1,
			CreationTime: types.Time(now.Add(1 * time.Hour)),
			UpdateTime:   types.Time(now.Add(5 * time.Hour)),
		},
		number("1100"): types.Order{},
		number("1200"): types.Order{},
		number("1300"): types.Order{
			OrderID:      3,
			CreationTime: types.Time(now.Add(3 * time.Hour)),
			UpdateTime:   types.Time(now.Add(6 * time.Hour)),
		},
		number("1400"): types.Order{
			OrderID:      2,
			CreationTime: types.Time(now.Add(2 * time.Hour)),
			UpdateTime:   types.Time(now.Add(4 * time.Hour)),
		},
	}

	orders := pinOrderMap.AscendingOrders()
	assert.Len(orders, 3)
	assert.Equal(uint64(2), orders[0].OrderID)
	assert.Equal(uint64(1), orders[1].OrderID)
	assert.Equal(uint64(3), orders[2].OrderID)
}

func Test_verifyFilledGrid(t *testing.T) {
	assert := assert.New(t)
	s := newTestStrategy()
	s.UpperPrice = number(400.0)
	s.LowerPrice = number(100.0)
	s.GridNum = 4
	s.grid = s.newGrid()

	t.Run("valid grid with buy/sell orders", func(t *testing.T) {
		pinOrderMap := PinOrderMap{
			number("100.00"): types.Order{
				OrderID: 1,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeBuy,
				},
			},
			number("200.00"): types.Order{},
			number("300.00"): types.Order{
				OrderID: 3,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeSell,
				},
			},
			number("400.00"): types.Order{
				OrderID: 4,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeSell,
				},
			},
		}

		assert.NoError(s.verifyFilledGrid(s.grid.Pins, pinOrderMap, nil))
	})
	t.Run("valid grid with only buy orders", func(t *testing.T) {
		pinOrderMap := PinOrderMap{
			number("100.00"): types.Order{
				OrderID: 1,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeBuy,
				},
			},
			number("200.00"): types.Order{
				OrderID: 2,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeBuy,
				},
			},
			number("300.00"): types.Order{
				OrderID: 3,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeBuy,
				},
			},
			number("400.00"): types.Order{},
		}

		assert.NoError(s.verifyFilledGrid(s.grid.Pins, pinOrderMap, nil))
	})
	t.Run("valid grid with only sell orders", func(t *testing.T) {
		pinOrderMap := PinOrderMap{
			number("100.00"): types.Order{},
			number("200.00"): types.Order{
				OrderID: 2,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeSell,
				},
			},
			number("300.00"): types.Order{
				OrderID: 3,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeSell,
				},
			},
			number("400.00"): types.Order{
				OrderID: 4,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeSell,
				},
			},
		}

		assert.NoError(s.verifyFilledGrid(s.grid.Pins, pinOrderMap, nil))
	})
	t.Run("invalid grid with multiple empty pins", func(t *testing.T) {
		pinOrderMap := PinOrderMap{
			number("100.00"): types.Order{
				OrderID: 1,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeBuy,
				},
			},
			number("200.00"): types.Order{},
			number("300.00"): types.Order{},
			number("400.00"): types.Order{
				OrderID: 4,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeSell,
				},
			},
		}

		assert.Error(s.verifyFilledGrid(s.grid.Pins, pinOrderMap, nil))
	})
	t.Run("invalid grid without empty pin", func(t *testing.T) {
		pinOrderMap := PinOrderMap{
			number("100.00"): types.Order{
				OrderID: 1,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeBuy,
				},
			},
			number("200.00"): types.Order{
				OrderID: 2,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeBuy,
				},
			},
			number("300.00"): types.Order{
				OrderID: 3,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeSell,
				},
			},
			number("400.00"): types.Order{
				OrderID: 4,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeSell,
				},
			},
		}

		assert.Error(s.verifyFilledGrid(s.grid.Pins, pinOrderMap, nil))
	})
	t.Run("invalid grid with Buy-empty-Sell-Buy order", func(t *testing.T) {
		pinOrderMap := PinOrderMap{
			number("100.00"): types.Order{
				OrderID: 1,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeBuy,
				},
			},
			number("200.00"): types.Order{},
			number("300.00"): types.Order{
				OrderID: 3,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeSell,
				},
			},
			number("400.00"): types.Order{
				OrderID: 4,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeBuy,
				},
			},
		}

		assert.Error(s.verifyFilledGrid(s.grid.Pins, pinOrderMap, nil))
	})
	t.Run("invalid grid with Sell-empty order", func(t *testing.T) {
		pinOrderMap := PinOrderMap{
			number("100.00"): types.Order{
				OrderID: 1,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeSell,
				},
			},
			number("200.00"): types.Order{
				OrderID: 2,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeSell,
				},
			},
			number("300.00"): types.Order{
				OrderID: 3,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeSell,
				},
			},
			number("400.00"): types.Order{},
		}

		assert.Error(s.verifyFilledGrid(s.grid.Pins, pinOrderMap, nil))
	})
	t.Run("invalid grid with empty-Buy order", func(t *testing.T) {
		pinOrderMap := PinOrderMap{
			number("100.00"): types.Order{},
			number("200.00"): types.Order{
				OrderID: 2,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeBuy,
				},
			},
			number("300.00"): types.Order{
				OrderID: 3,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeBuy,
				},
			},
			number("400.00"): types.Order{
				OrderID: 4,
				SubmitOrder: types.SubmitOrder{
					Side: types.SideTypeBuy,
				},
			},
		}

		assert.Error(s.verifyFilledGrid(s.grid.Pins, pinOrderMap, nil))
	})

}
*/
