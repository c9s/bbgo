package bbgo

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo/service"
	"github.com/c9s/bbgo/pkg/util"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo/exchange/binance"
	"github.com/c9s/bbgo/pkg/bbgo/types"
)

type Strategy interface {
	Init(tradingContext *TradingContext, trader types.Trader) error
	OnNewStream(stream *types.StandardPrivateStream) error
}

type Trader struct {
	Notifier *SlackNotifier

	// Context is trading Context
	Context *TradingContext

	Exchange *binance.Exchange

	reportTimer *time.Timer

	ProfitAndLossCalculator *ProfitAndLossCalculator

	TradeService *service.TradeService
}

func (trader *Trader) RunStrategy(ctx context.Context, strategy Strategy) (chan struct{}, error) {
	symbol := trader.Context.Symbol

	balances, err := trader.Exchange.QueryAccountBalances(ctx)
	if err != nil {
		return nil, err
	}

	trader.Context.Balances = balances
	for _, balance := range balances {
		if util.NotZero(balance.Available) {
			log.Infof("[trader] balance %s %f", balance.Currency, balance.Available)
		}
	}

	if err := strategy.Init(trader.Context, trader); err != nil {
		return nil, err
	}

	stream, err := trader.Exchange.NewPrivateStream()
	if err != nil {
		return nil, err
	}

	// bind kline store to the stream
	klineStore := NewKLineStore()
	klineStore.BindPrivateStream(&stream.StandardPrivateStream)

	if err := strategy.OnNewStream(&stream.StandardPrivateStream); err != nil {
		return nil, err
	}

	trader.reportTimer = time.AfterFunc(1*time.Second, func() {
		trader.ReportPnL()
	})

	stream.OnTrade(func(trade *types.Trade) {
		if trade.Symbol != symbol {
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

	stream.OnBalanceSnapshot(func(snapshot map[string]types.Balance) {
		trader.Context.Lock()
		defer trader.Context.Unlock()
		for _, balance := range snapshot {
			trader.Context.Balances[balance.Currency] = balance
		}
	})

	// stream.OnOutboundAccountInfoEvent(func(e *binance.OutboundAccountInfoEvent) { })
	stream.OnBalanceUpdateEvent(func(e *binance.BalanceUpdateEvent) {
		trader.Context.Lock()
		defer trader.Context.Unlock()
		delta := util.MustParseFloat(e.Delta)
		if balance, ok := trader.Context.Balances[e.Asset]; ok {
			balance.Available += delta
			trader.Context.Balances[e.Asset] = balance
		}
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
