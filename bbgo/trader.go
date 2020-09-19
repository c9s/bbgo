package bbgo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo/accounting"
	"github.com/c9s/bbgo/pkg/bbgo/config"
	"github.com/c9s/bbgo/pkg/bbgo/notifier/slacknotifier"
	"github.com/c9s/bbgo/pkg/bbgo/service"

	"github.com/c9s/bbgo/pkg/bbgo/exchange/binance"
	"github.com/c9s/bbgo/pkg/bbgo/types"
)

type Strategy interface {
	Load(tradingContext *Context, trader types.Trader) error
	OnNewStream(stream *types.StandardPrivateStream) error
}

type Trader struct {
	Symbol       string
	TradeService *service.TradeService
	TradeSync    *service.TradeSync

	Notifier *slacknotifier.Notifier

	// Context is trading Context
	Context *Context

	Exchange *binance.Exchange

	reportTimer *time.Timer

	ProfitAndLossCalculator *accounting.ProfitAndLossCalculator

	Account *Account
}

func NewTrader(db *sqlx.DB, exchange *binance.Exchange, symbol string) *Trader {
	tradeService := &service.TradeService{DB: db}
	return &Trader{
		Symbol:       symbol,
		Exchange:     exchange,
		TradeService: tradeService,
		TradeSync: &service.TradeSync{
			Service:  tradeService,
			Exchange: exchange,
		},
	}
}

func (trader *Trader) Initialize(ctx context.Context, startTime time.Time) error {

	log.Info("syncing trades from exchange...")
	if err := trader.TradeSync.Sync(ctx, trader.Symbol, startTime); err != nil {
		return err
	}

	var err error
	var trades []types.Trade
	tradingFeeCurrency := trader.Exchange.TradingFeeCurrency()
	if strings.HasPrefix(trader.Symbol, tradingFeeCurrency) {
		trades, err = trader.TradeService.QueryForTradingFeeCurrency(trader.Symbol, tradingFeeCurrency)
	} else {
		trades, err = trader.TradeService.Query(trader.Symbol)
	}

	if err != nil {
		return err
	}

	log.Infof("%d trades loaded", len(trades))

	stockManager := &StockManager{
		Symbol:             trader.Symbol,
		TradingFeeCurrency: tradingFeeCurrency,
	}

	checkpoints, err := stockManager.AddTrades(trades)
	if err != nil {
		return err
	}

	log.Infof("found checkpoints: %+v", checkpoints)

	market, ok := types.FindMarket(trader.Symbol)
	if !ok {
		return fmt.Errorf("%s market not found", trader.Symbol)
	}

	currentPrice, err := trader.Exchange.QueryAveragePrice(ctx, trader.Symbol)
	if err != nil {
		return err
	}

	trader.Context = &Context{
		CurrentPrice: currentPrice,
		Symbol:       trader.Symbol,
		Market:       market,
		StockManager: stockManager,
	}

	/*
		if len(checkpoints) > 0 {
			// get the last checkpoint
			idx := checkpoints[len(checkpoints)-1]
			if idx < len(trades)-1 {
				trades = trades[idx:]
				firstTrade := trades[0]
				pnlStartTime = firstTrade.Time
				notifier.Notify("%s Found the latest trade checkpoint %s", firstTrade.Symbol, firstTrade.Time, firstTrade)
			}
		}
	*/

	trader.ProfitAndLossCalculator = &accounting.ProfitAndLossCalculator{
		TradingFeeCurrency: tradingFeeCurrency,
		Symbol:             trader.Symbol,
		StartTime:          startTime,
		CurrentPrice:       currentPrice,
		Trades:             trades,
	}

	account, err := LoadAccount(ctx, trader.Exchange)
	if err != nil {
		return err
	}

	trader.Account = account
	trader.Context.Balances = account.Balances
	account.Print()

	return nil
}

func (trader *Trader) RunStrategyWithHotReload(ctx context.Context, strategy Strategy, configFile string) (chan struct{}, error) {
	var done = make(chan struct{})
	var configWatcherDone = make(chan struct{})

	log.Infof("watching config file: %v", configFile)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	defer watcher.Close()

	if err := watcher.Add(configFile); err != nil {
		return nil, err
	}

	go func() {
		strategyContext, strategyCancel := context.WithCancel(ctx)
		defer strategyCancel()
		defer close(done)

		traderDone, err := trader.RunStrategy(strategyContext, strategy)
		if err != nil {
			return
		}

		var configReloadTimer *time.Timer = nil
		defer close(configWatcherDone)

		for {
			select {

			case <-ctx.Done():
				return

			case <-traderDone:
				log.Infof("reloading config file %s", configFile)
				if err := config.LoadConfigFile(configFile, strategy); err != nil {
					log.WithError(err).Error("error load config file")
				}

				trader.Notifier.Notify("config reloaded, restarting trader")

				traderDone, err = trader.RunStrategy(strategyContext, strategy)
				if err != nil {
					log.WithError(err).Error("[trader] error:", err)
					return
				}

			case event := <-watcher.Events:
				log.Infof("[fsnotify] event: %+v", event)

				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Info("[fsnotify] modified file:", event.Name)
				}

				if configReloadTimer != nil {
					configReloadTimer.Stop()
				}

				configReloadTimer = time.AfterFunc(3*time.Second, func() {
					strategyCancel()
				})

			case err := <-watcher.Errors:
				log.WithError(err).Error("[fsnotify] error:", err)
				return

			}
		}
	}()

	return done, nil
}

func (trader *Trader) RunStrategy(ctx context.Context, strategy Strategy) (chan struct{}, error) {
	if err := strategy.Load(trader.Context, trader); err != nil {
		return nil, err
	}

	stream, err := trader.Exchange.NewPrivateStream()
	if err != nil {
		return nil, err
	}

	// bind kline store to the stream
	klineStore := NewMarketDataStore()
	klineStore.BindPrivateStream(&stream.StandardPrivateStream)

	trader.Account.BindPrivateStream(stream)

	if err := strategy.OnNewStream(&stream.StandardPrivateStream); err != nil {
		return nil, err
	}

	trader.reportTimer = time.AfterFunc(1*time.Second, func() {
		trader.reportPnL()
	})

	stream.OnTrade(func(trade *types.Trade) {
		if trade.Symbol != trader.Symbol {
			return
		}

		if err := trader.TradeService.Insert(*trade); err != nil {
			log.WithError(err).Error("trade insert error")
		}

		trader.Notifier.NotifyTrade(trade)
		trader.ProfitAndLossCalculator.AddTrade(*trade)
		_, err := trader.Context.StockManager.AddTrades([]types.Trade{*trade})
		if err != nil {
			log.WithError(err).Error("stock manager load trades error")
		}

		if trader.reportTimer != nil {
			trader.reportTimer.Stop()
		}

		trader.reportTimer = time.AfterFunc(1*time.Minute, func() {
			trader.reportPnL()
		})
	})

	stream.OnKLineClosed(func(kline types.KLine) {
		trader.ProfitAndLossCalculator.SetCurrentPrice(kline.Close)
		trader.Context.SetCurrentPrice(kline.Close)
	})

	var eventC = make(chan interface{}, 20)
	if err := stream.Connect(ctx, eventC); err != nil {
		return nil, err
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		defer stream.Close()

		for {
			select {

			case <-ctx.Done():
				return

			// drain the event channel
			case <-eventC:

			}
		}
	}()

	return done, nil
}

func (trader *Trader) reportPnL() {
	report := trader.ProfitAndLossCalculator.Calculate()
	report.Print()
	trader.Notifier.NotifyPnL(report)
}

func (trader *Trader) SubmitOrder(ctx context.Context, order *types.SubmitOrder) {
	trader.Notifier.Notify(":memo: Submitting %s %s %s order with quantity: %s", order.Symbol, order.Type, order.Side, order.QuantityString, order)

	orderProcessor := &OrderProcessor{
		MinQuoteBalance: 0,
		MaxAssetBalance: 0,
		MinAssetBalance: 0,
		MinProfitSpread: 0,
		MaxOrderAmount:  0,
		Exchange:        trader.Exchange,
		Trader:          trader,
	}

	err := orderProcessor.Submit(ctx, order)

	if err != nil {
		log.WithError(err).Errorf("order create error: side %s quantity: %s", order.Side, order.QuantityString)
		return
	}
}
