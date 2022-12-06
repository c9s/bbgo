//go:build !dnum

package grid2

import (
	"context"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/backtest"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
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
	}

	s := &Strategy{
		logger:          logrus.NewEntry(logrus.New()),
		Market:          market,
		GridProfitStats: newGridProfitStats(market),
		UpperPrice:      number(20_000),
		LowerPrice:      number(10_000),
		GridNum:         10,

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

func RunBacktest(t *testing.T, strategy bbgo.SingleExchangeStrategy) {
	// TEMPLATE {{{ start backtest
	const sqliteDbFile = "../../../data/bbgo_test.sqlite3"
	const backtestExchangeName = "binance"
	const backtestStartTime = "2022-06-01"
	const backtestEndTime = "2022-06-30"

	startTime, err := types.ParseLooseFormatTime(backtestStartTime)
	assert.NoError(t, err)

	endTime, err := types.ParseLooseFormatTime(backtestEndTime)
	assert.NoError(t, err)

	backtestConfig := &bbgo.Backtest{
		StartTime:    startTime,
		EndTime:      &endTime,
		RecordTrades: false,
		FeeMode:      bbgo.BacktestFeeModeToken,
		Accounts: map[string]bbgo.BacktestAccount{
			backtestExchangeName: {
				MakerFeeRate: number(0.075 * 0.01),
				TakerFeeRate: number(0.075 * 0.01),
				Balances: bbgo.BacktestAccountBalanceMap{
					"USDT": number(10_000.0),
					"BTC":  number(1.0),
				},
			},
		},
		Symbols:       []string{"BTCUSDT"},
		Sessions:      []string{backtestExchangeName},
		SyncSecKLines: false,
	}

	t.Logf("backtestConfig: %+v", backtestConfig)

	ctx := context.Background()
	environ := bbgo.NewEnvironment()
	environ.SetStartTime(startTime.Time())

	info, err := os.Stat(sqliteDbFile)
	assert.NoError(t, err)
	t.Logf("sqlite: %+v", info)

	err = environ.ConfigureDatabaseDriver(ctx, "sqlite3", sqliteDbFile)
	if !assert.NoError(t, err) {
		return
	}

	backtestService := &service.BacktestService{DB: environ.DatabaseService.DB}
	defer func() {
		err := environ.DatabaseService.DB.Close()
		assert.NoError(t, err)
	}()

	environ.BacktestService = backtestService
	bbgo.SetBackTesting(backtestService)
	defer bbgo.SetBackTesting(nil)

	exName, err := types.ValidExchangeName(backtestExchangeName)
	if !assert.NoError(t, err) {
		return
	}

	t.Logf("using exchange source: %s", exName)

	publicExchange, err := exchange.NewPublic(exName)
	if !assert.NoError(t, err) {
		return
	}

	backtestExchange, err := backtest.NewExchange(exName, publicExchange, backtestService, backtestConfig)
	if !assert.NoError(t, err) {
		return
	}

	session := environ.AddExchange(backtestExchangeName, backtestExchange)
	assert.NotNil(t, session)

	err = environ.Init(ctx)
	assert.NoError(t, err)

	for _, ses := range environ.Sessions() {
		userDataStream := ses.UserDataStream.(types.StandardStreamEmitter)
		backtestEx := ses.Exchange.(*backtest.Exchange)
		backtestEx.MarketDataStream = ses.MarketDataStream.(types.StandardStreamEmitter)
		backtestEx.BindUserData(userDataStream)
	}

	trader := bbgo.NewTrader(environ)
	if assert.NotNil(t, trader) {
		trader.DisableLogging()
	}

	userConfig := &bbgo.Config{
		Backtest: backtestConfig,
		ExchangeStrategies: []bbgo.ExchangeStrategyMount{
			{
				Mounts:   []string{backtestExchangeName},
				Strategy: strategy,
			},
		},
	}

	err = trader.Configure(userConfig)
	assert.NoError(t, err)

	err = trader.Run(ctx)
	assert.NoError(t, err)

	allKLineIntervals, requiredInterval, backTestIntervals := backtest.CollectSubscriptionIntervals(environ)
	t.Logf("requiredInterval: %s backTestIntervals: %v", requiredInterval, backTestIntervals)

	_ = allKLineIntervals
	exchangeSources, err := backtest.InitializeExchangeSources(environ.Sessions(), startTime.Time(), endTime.Time(), requiredInterval, backTestIntervals...)
	if !assert.NoError(t, err) {
		return
	}

	doneC := make(chan struct{})
	go func() {
		count := 0
		exSource := exchangeSources[0]
		for k := range exSource.C {
			exSource.Exchange.ConsumeKLine(k, requiredInterval)
			count++
		}

		err = exSource.Exchange.CloseMarketData()
		assert.NoError(t, err)

		assert.Greater(t, count, 0, "kLines count must be greater than 0, please check your backtest date range and symbol settings")

		close(doneC)
	}()

	<-doneC
	// }}}
}
