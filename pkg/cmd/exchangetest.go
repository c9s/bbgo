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
	"github.com/c9s/bbgo/pkg/types"
)

func checked(msg, desc string, args ...any) {
	log.Infof(fmt.Sprintf(msg+": ✅ "+desc, args...))
}

func assert(condition bool, msg string, args ...any) bool {
	if !condition {
		log.Errorf("assertion failed: ❗️ "+msg, args...)
		return false
	} else {
		log.Infof("assertion passed: ✅ "+msg, args...)
		return true
	}
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
				log.Infof("market trade: %+v", trade)
			})
			marketDataStream.OnKLine(func(kline types.KLine) {
				klineCount++
				klineSamples = append(klineSamples, kline)
				log.Infof("kline: %+v", kline)
			})
			marketDataStream.OnBookSnapshot(func(book types.SliceOrderBook) {
				bookSnapshotCount++
				bookSnapshotSamples = append(bookSnapshotSamples, book)
				log.Infof("book snapshot: %+v", book)
			})
			marketDataStream.OnBookUpdate(func(book types.SliceOrderBook) {
				bookUpdateCount++
				bookUpdateSamples = append(bookUpdateSamples, book)
				log.Infof("book update: %+v", book)
			})

			marketDataCtx, cancelMarketData := context.WithCancel(ctx)
			defer cancelMarketData()

			marketDataDone := make(chan struct{})
			go func() {
				defer close(marketDataDone)
				select {
				case <-marketDataCtx.Done():
					return
				case <-time.After(3 * time.Minute):
					cancelMarketData()
				}
			}()

			err := marketDataStream.Connect(marketDataCtx)
			if noError(err, "marketDataStream.Connect") {
				<-marketDataDone
			
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
				if len(tradeSamples) > 0 {
					for _, t := range tradeSamples {
						if t.Price.Sign() < 0 {
							log.Errorf("invalid trade price: %+v", t)
						}
					}
				}
				if len(klineSamples) > 0 {
					for _, k := range klineSamples {
						if k.Close.Sign() < 0 {
							log.Errorf("invalid kline close price: %+v", k)
						}
					}
				}
				if len(bookSnapshotSamples) > 0 {
					for _, b := range bookSnapshotSamples {
						if len(b.Bids) == 0 && len(b.Asks) == 0 {
							log.Errorf("empty book snapshot: %+v", b)
						}
					}
				}
				if len(bookUpdateSamples) > 0 {
					for _, b := range bookUpdateSamples {
						if len(b.Bids) == 0 && len(b.Asks) == 0 {
							log.Errorf("empty book update: %+v", b)
						}
					}
				}
			}
		} else {
			log.Warnf("types.Exchange is not implemented, some tests will be skipped")
		}

		cmdutil.WaitForSignal(ctx, syscall.SIGINT, syscall.SIGTERM)
		cancel()

		return nil
	},
}

func init() {
	exchangeTestCmd.Flags().String("exchange", "", "the exchange name to test")
	exchangeTestCmd.MarkFlagRequired("exchange")

	RootCmd.AddCommand(exchangeTestCmd)
}
