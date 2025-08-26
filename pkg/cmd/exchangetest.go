package cmd

import (
	"context"
	"fmt"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/cmd/cmdutil"
	"github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	exchangeTestCmd.Flags().String("exchange", "", "the exchange name to test")
	exchangeTestCmd.MarkFlagRequired("exchange")

	RootCmd.AddCommand(exchangeTestCmd)
}

var exchangeTestCmd = &cobra.Command{
	Use:   "exchange-test [--session SESSION]",
	Short: "test the exchange",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		exchangeNameStr, err := cmd.Flags().GetString("exchange")
		if err != nil {
			return err
		}

		exchangeName, err := types.ValidExchangeName(exchangeNameStr)
		if err != nil {
			return err
		}

		if !viper.IsSet(string(exchangeName) + "-api-key") {
			return fmt.Errorf("api key is not set for exchange %s", exchangeName)
		}

		exMinimal, err := exchange.NewWithEnvVarPrefix(exchangeName, "")
		if err != nil {
			return err
		}

		checked("types.ExchangeMinimal interface", "minimal exchange interface is implemented")

		environ := bbgo.NewEnvironment()

		if service, ok := exMinimal.(types.ExchangeAccountService); ok {
			checked("types.ExchangeAccountService", fmt.Sprintf("(%T)", service))

			bals, err := service.QueryAccountBalances(ctx)
			if noError(err, "QueryAccountBalances", bals) {
				checked("QueryAccountBalances", fmt.Sprintf("found %d balances", len(bals)))
				for cu, b := range bals {
					assert(b.Currency == cu, "balance currency %s matches map key %s", b.Currency, cu)
					assert(b.Total().Sign() > 0, "balance %s is positive", b.Currency)
				}
			}

			account, err := service.QueryAccount(ctx)
			if noError(err, "QueryAccount", account) {
				checked("QueryAccount", "account query successful")

				account.Print(log.Infof)
				assert(len(account.Balances()) > 0, "account has balances: %d", len(account.Balances()))
				assert(account.AccountType != "", "account type is set to %q", account.AccountType)
				switch account.AccountType {
				case types.AccountTypeMargin:
					assert(!account.MarginLevel.IsZero(), "margin level should not be zero: %s", account.MarginLevel)
					assert(!account.MarginRatio.IsZero(), "margin ratio should not be zero: %s", account.MarginRatio)
				}
			}
		} else {
			log.Warnf("types.ExchangeAccountService is not implemented")
		}

		if service, ok := exMinimal.(types.ExchangeMarketDataService); ok {
			checked("types.ExchangeMarketDataService", "(%T)", service)

			markets, err := service.QueryMarkets(ctx)
			if noError(err, "QueryMarkets") {
				checked("QueryMarkets", "markets query successful: %d markets", len(markets))
				if assert(len(markets) > 0, "markets are available: %d", len(markets)) {
					selectedMarkets := markets.FindAssetMarkets("BTC", "ETH")
					for sym, m := range selectedMarkets {
						failed := false
						failed = failed || !assert(m.Symbol == sym, "market symbol matches map key: %s", m.Symbol)
						failed = failed || !assert(m.Symbol != "", "market symbol is not empty: %s", m.Symbol)
						failed = failed || !assert(m.BaseCurrency != "", "%s market base currency is not empty: %s", m.Symbol, m.BaseCurrency)
						failed = failed || !assert(m.QuoteCurrency != "", "%s market quote currency is not empty: %s", m.Symbol, m.QuoteCurrency)
						failed = failed || !assert(m.TickSize.Sign() > 0, "%s market price step is positive: %s", m.Symbol, m.TickSize)
						failed = failed || !assert(m.StepSize.Sign() > 0, "%s market quantity step is positive: %s", m.Symbol, m.StepSize)
						failed = failed || !assert(m.PricePrecision > 0, "%s market price precision is positive: %d", m.Symbol, m.PricePrecision)
						failed = failed || !assert(m.VolumePrecision > 0, "%s market volume precision is positive: %d", m.Symbol, m.VolumePrecision)
						if failed {
							log.Errorf("invalid market: %+v", m)
						}
					}
				}
			}

			ticker, err := service.QueryTicker(ctx, "BTCUSDT")
			if noError(err, "QueryTicker", ticker) {
				checked("QueryTicker", "ticker query successful")
				assert(ticker.GetValidPrice().Sign() > 0, "ticker last price is positive: %s", ticker.GetValidPrice())
				assert(ticker.Buy.Sign() > 0, "ticker buy price is positive: %s", ticker.Buy)
				assert(ticker.Sell.Sign() > 0, "ticker sell price is positive: %s", ticker.Sell)
			}

			tickers, err := service.QueryTickers(ctx, "BTCUSDT", "ETHUSDT", "ETHBTC")
			if noError(err, "QueryTickers") {
				checked("QueryTickers", "tickers query successful: %d tickers", len(tickers))
				assert(len(tickers) >= 2, "at least 2 tickers are returned: %d", len(tickers))
				for sym, t := range tickers {
					failed := false
					failed = failed || !assert(t.GetValidPrice().Sign() > 0, "%s ticker last price is positive: %s", sym, t.GetValidPrice())
					failed = failed || !assert(t.Buy.Sign() > 0, "%s ticker buy price is positive: %s", sym, t.Buy)
					failed = failed || !assert(t.Sell.Sign() > 0, "%s ticker sell price is positive: %s", sym, t.Sell)
					if failed {
						log.Errorf("invalid ticker: %+v", t)
					}
				}
			}

		} else {
			log.Warnf("types.ExchangeMarketDataService is not implemented")
		}

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		if ex, ok := exMinimal.(types.Exchange); ok {
			environ.AddExchange(exchangeName.String(), ex)
			checked("types.Exchange", "the Exchange interface is implemented")
			testMarketDataStream(ctx, ex)
			testUserDataStream(ctx, ex)
		} else {
			log.Warnf("types.Exchange is not implemented, some tests will be skipped")
		}

		log.Infof("exchange test for %s completed, press Ctrl+C to exit", exchangeName)
		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		cancel()

		return nil
	},
}

func noError(err error, title string, values ...any) bool {
	if err != nil {
		log.Errorf("errored: ❗️ %s: %s", title, err)
		return false
	} else {
		if len(values) > 0 {
			log.Infof("%s passed: %+v", title, values[0])
		} else {
			log.Infof("%s passed: ✅", title)
		}

		return true
	}
}

func assert(condition bool, msg string, args ...any) bool {
	if condition {
		log.Infof("assertion passed: ✅ "+msg, args...)
		return true
	} else {
		log.Errorf("assertion failed: ❗️ "+msg, args...)
		return false
	}
}

// testMarketDataStream tests the market data stream for the given exchange.
func testMarketDataStream(ctx context.Context, ex types.Exchange) {
	marketDataStream := ex.NewStream()
	marketDataStream.SetPublicOnly()
	marketDataStream.Subscribe(types.BookChannel, "BTCUSDT", types.SubscribeOptions{
		Depth: types.DepthLevelFull,
	})
	marketDataStream.Subscribe(types.KLineChannel, "BTCUSDT", types.SubscribeOptions{
		Interval: types.Interval1m,
	})
	marketDataStream.Subscribe(types.MarketTradeChannel, "BTCUSDT", types.SubscribeOptions{})

	tradeCount := 0
	klineCount := 0
	bookSnapshotCount := 0
	bookUpdateCount := 0

	var tradeSamples []types.Trade
	var klineSamples []types.KLine
	var bookSnapshotSamples []types.SliceOrderBook
	var bookUpdateSamples []types.SliceOrderBook

	marketDataStream.OnMarketTrade(func(trade types.Trade) {
		tradeCount++
		tradeSamples = append(tradeSamples, trade)
		if err := trade.Validate(); err != nil {
			log.Errorf("invalid trade: %s, trade: %+v", err, trade)
		}
	})
	marketDataStream.OnKLine(func(kline types.KLine) {
		klineCount++
		klineSamples = append(klineSamples, kline)
		if err := kline.Validate(); err != nil {
			log.Errorf("invalid kline: %s, kline: %+v", err, kline)
		}
	})
	marketDataStream.OnBookSnapshot(func(book types.SliceOrderBook) {
		bookSnapshotCount++
		bookSnapshotSamples = append(bookSnapshotSamples, book)
		if err := book.Validate(); err != nil {
			log.Errorf("invalid book snapshot: %s, book: %+v", err, book)
		}
	})
	marketDataStream.OnBookUpdate(func(book types.SliceOrderBook) {
		bookUpdateCount++
		bookUpdateSamples = append(bookUpdateSamples, book)
		if err := book.Validate(); err != nil {
			log.Errorf("invalid book update: %s, book: %+v", err, book)
		}
	})

	marketDataCtx, cancelMarketData := context.WithCancel(ctx)
	defer cancelMarketData()

	collectingTime := 3 * time.Minute
	marketDataDone := make(chan struct{})
	go func() {
		defer close(marketDataDone)
		select {
		case <-marketDataCtx.Done():
			return
		case <-time.After(collectingTime):
			cancelMarketData()
			return
		}
	}()

	err := marketDataStream.Connect(marketDataCtx)
	if noError(err, "marketDataStream.Connect") {
		log.Infof("collecting market data for %s...", collectingTime)
		select {
		case <-ctx.Done():
			return
		case <-marketDataDone:
		}
		log.Infof("market data collection done: trades=%d, klines=%d, bookSnapshots=%d, bookUpdates=%d", tradeCount, klineCount, bookSnapshotCount, bookUpdateCount)

		// check statistics and data correctness
		if tradeCount == 0 {
			log.Errorf("no market trade received")
		}
		if klineCount == 0 {
			log.Errorf("no kline received")
		}
		if bookSnapshotCount == 0 {
			log.Errorf("no book snapshot received")
		}
		if bookUpdateCount == 0 {
			log.Errorf("no book update received")
		}

		// check sample data correctness
		if assert(len(tradeSamples) > 0, "trade samples are not empty") {
			assert(tradeCount > 0, "trade count is positive")
			for _, t := range tradeSamples {
				if err := t.Validate(); err != nil {
					log.Errorf("invalid trade: %s, trade: %+v", err, t)
				}
			}
		}

		if assert(len(klineSamples) > 0, "kline samples are not empty") {
			assert(klineCount > 0, "kline count is positive")
			for _, k := range klineSamples {
				if err := k.Validate(); err != nil {
					log.Errorf("invalid kline: %s, kline: %+v", err, k)
				}
			}
		}

		if assert(len(bookSnapshotSamples) > 0, "book snapshot samples are not empty") {
			assert(bookSnapshotCount > 0, "book snapshot count is positive")
			for _, b := range bookSnapshotSamples {
				if err := b.Validate(); err != nil {
					log.Errorf("invalid book snapshot: %s, book: %+v", err, b)
				}
			}
		}

		if assert(len(bookUpdateSamples) > 0, "book update samples are not empty") {
			assert(bookUpdateCount > 0, "book update count is positive")
			for _, b := range bookUpdateSamples {
				if err := b.Validate(); err != nil {
					log.Errorf("invalid book update: %s, book: %+v", err, b)
				}
			}
		}
	}

	log.Infof("market data collected: trades=%d, klines=%d, bookSnapshots=%d, bookUpdates=%d", tradeCount, klineCount, bookSnapshotCount, bookUpdateCount)
}

func checked(msg, desc string, args ...any) {
	log.Infof(fmt.Sprintf(msg+": ✅ "+desc, args...))
}

func testUserDataStream(ctx context.Context, ex types.Exchange) {
	userDataStream := ex.NewStream()

	orderCount := 0
	tradeCount := 0
	balanceCount := 0

	var orderSamples []types.Order
	var tradeSamples []types.Trade
	var balanceSamples []types.BalanceMap

	userDataStream.OnOrderUpdate(func(order types.Order) {
		orderCount++
		orderSamples = append(orderSamples, order)
		if err := order.Validate(); err != nil {
			log.Errorf("invalid websocket order update: %s, order: %+v", err, order)
		}
	})

	userDataStream.OnTradeUpdate(func(trade types.Trade) {
		tradeCount++
		tradeSamples = append(tradeSamples, trade)
		if err := trade.Validate(); err != nil {
			log.Errorf("invalid websocket trade: %s, trade: %+v", err, trade)
		}
	})

	userDataStream.OnBalanceUpdate(func(bals types.BalanceMap) {
		balanceCount++
		balanceSamples = append(balanceSamples, bals)
		for _, balance := range bals {
			if err := balance.Validate(); err != nil {
				log.Errorf("invalid websocket balance: %s, balance: %+v", err, balance)
			}
		}
	})

	userDataCtx, cancelUserData := context.WithCancel(ctx)
	defer cancelUserData()

	userDataDone := make(chan struct{})
	go func() {
		defer close(userDataDone)
		select {
		case <-userDataCtx.Done():
			return
		case <-time.After(3 * time.Minute):
			cancelUserData()
			return
		}
	}()

	err := userDataStream.Connect(userDataCtx)
	if noError(err, "userDataStream.Connect") {
		// submit order
		ticker, err := ex.QueryTicker(ctx, "BTCUSDT")
		if !noError(err, "QueryTicker", ticker) {
			return
		}
		noError(ticker.Validate(), "ticker.Validate")

		openOrders, err := ex.QueryOpenOrders(ctx, "BTCUSDT")
		if !noError(err, "QueryOpenOrders", openOrders) {
			return
		}

		if len(openOrders) > 0 {
			err := ex.CancelOrders(ctx, openOrders...)
			if !noError(err, "CancelOrders", openOrders) {
				return
			} else {
				// wait for some time to let the orders be cancelled
				time.Sleep(5 * time.Second)
			}
		}

		testAmount := fixedpoint.NewFromFloat(10.0)
		qty := testAmount.Div(ticker.Sell)

		createdOrder1, err := ex.SubmitOrder(ctx, types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Price:    ticker.Sell,
			Quantity: qty,
		})
		if !noError(err, "SubmitOrder with Limit Order", createdOrder1) {
			return
		}

		noError(createdOrder1.Validate(), "createdOrder1.Validate (RESTful API)")

		time.Sleep(5 * time.Second)

		createdOrder2, err := ex.SubmitOrder(ctx, types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeMarket,
			Quantity: qty,
		})
		if !noError(err, "SubmitOrder with Market Order", createdOrder2) {
			return
		}

		noError(createdOrder2.Validate(), "createdOrder2.Validate (RESTful API)")

		if service, ok := ex.(types.ExchangeOrderQueryService); ok {
			checked("types.ExchangeOrderQueryService", "(%T)", service)

			trades1, err := service.QueryOrderTrades(ctx, createdOrder1.AsQuery())
			if noError(err, "QueryOrderTrades for createdOrder1", trades1) {
				assert(len(trades1) > 0, "trades1 is not empty")
				for _, trade := range trades1 {
					if err := trade.Validate(); err != nil {
						log.Errorf("invalid trade for createdOrder1: %s, trade: %+v", err, trade)
					}
				}
			}

			trades2, err := service.QueryOrderTrades(ctx, createdOrder2.AsQuery())
			if noError(err, "QueryOrderTrades for createdOrder2", trades2) {
				assert(len(trades2) > 0, "at least one trade for market order createdOrder2")
				for _, trade := range trades2 {
					if err := trade.Validate(); err != nil {
						log.Errorf("invalid trade for createdOrder2: %s, trade: %+v", err, trade)
					}
				}
			}
		}

		time.Sleep(5 * time.Second)
		cancelUserData()

		// wait
		select {
		case <-ctx.Done():
			return
		case <-userDataDone:

		}
		log.Infof("user data collection done: orders=%d, balances=%d", orderCount, balanceCount)

		// check statistics and data correctness
		assert(len(orderSamples) > 0, "order samples are not empty")
		assert(orderCount > 0, "order count is positive")

		assert(len(tradeSamples) > 0, "trade samples are not empty")
		assert(tradeCount > 0, "trade count is positive")

		assert(len(balanceSamples) > 0, "balance samples are not empty")
		assert(balanceCount > 0, "balance count is positive")
	}
	log.Infof("user data collected: orders=%d, balances=%d", orderCount, balanceCount)
}
