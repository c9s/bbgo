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
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

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

func TestBacktestStrategy(t *testing.T) {
	if v, ok := util.GetEnvVarBool("TEST_BACKTEST"); !ok || !v {
		t.Skip("backtest flag is required")
		return
	}

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
