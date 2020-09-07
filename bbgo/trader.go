package bbgo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"

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

	Notifier *SlackNotifier

	// Context is trading Context
	Context *Context

	Exchange *binance.Exchange

	reportTimer *time.Timer

	ProfitAndLossCalculator *ProfitAndLossCalculator

	Account *Account
}

func NewTrader(db *sqlx.DB, exchange *binance.Exchange, symbol string) *Trader {
	tradeService := &service.TradeService{DB: db}
	tradeSync := &service.TradeSync{Service: tradeService, Exchange: exchange}
	return &Trader{
		Symbol:       symbol,
		TradeService: tradeService,
		TradeSync:    tradeSync,
	}
}

func (trader *Trader) Initialize(ctx context.Context, startTime time.Time) error {

	log.Info("syncing trades...")
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

	trader.ProfitAndLossCalculator = &ProfitAndLossCalculator{
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

func (trader *Trader) RunStrategy(ctx context.Context, strategy Strategy) (chan struct{}, error) {
	if err := strategy.Load(trader.Context, trader); err != nil {
		return nil, err
	}

	stream, err := trader.Exchange.NewPrivateStream()
	if err != nil {
		return nil, err
	}

	// bind kline store to the stream
	klineStore := NewKLineStore()
	klineStore.BindPrivateStream(&stream.StandardPrivateStream)

	trader.Account.BindPrivateStream(stream)

	if err := strategy.OnNewStream(&stream.StandardPrivateStream); err != nil {
		return nil, err
	}

	trader.reportTimer = time.AfterFunc(1*time.Second, func() {
		trader.ReportPnL()
	})

	stream.OnTrade(func(trade *types.Trade) {
		if trade.Symbol != trader.Symbol {
			return
		}

		if err := trader.TradeService.Insert(*trade); err != nil {
			log.WithError(err).Error("trade insert error")
		}

		trader.ReportTrade(trade)
		trader.ProfitAndLossCalculator.AddTrade(*trade)
		_, err := trader.Context.StockManager.AddTrades([]types.Trade{*trade})
		if err != nil {
			log.WithError(err).Error("stock manager load trades error")
		}

		if trader.reportTimer != nil {
			trader.reportTimer.Stop()
		}

		trader.reportTimer = time.AfterFunc(1*time.Minute, func() {
			trader.ReportPnL()
		})
	})

	stream.OnKLineEvent(func(e *binance.KLineEvent) {
		trader.ProfitAndLossCalculator.SetCurrentPrice(e.KLine.GetClose())
		trader.Context.SetCurrentPrice(e.KLine.GetClose())
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

func (trader *Trader) ReportTrade(trade *types.Trade) {
	trader.Notifier.ReportTrade(trade)
}

func (trader *Trader) ReportPnL() {
	report := trader.ProfitAndLossCalculator.Calculate()
	report.Print()
	trader.Notifier.ReportPnL(report)
}

func (trader *Trader) SubmitOrder(ctx context.Context, order *types.SubmitOrder) {
	trader.Notifier.Notify(":memo: Submitting %s %s %s order with quantity: %s", order.Symbol, order.Type, order.Side, order.Quantity, order)

	err := trader.Exchange.SubmitOrder(ctx, order)
	if err != nil {
		log.WithError(err).Errorf("order create error: side %s quantity: %s", order.Side, order.Quantity)
		return
	}
}
