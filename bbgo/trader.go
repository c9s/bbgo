package bbgo

import (
	"context"
	"github.com/c9s/bbgo/pkg/util"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo/exchange/binance"
	"github.com/c9s/bbgo/pkg/bbgo/types"
)

type Strategy interface {
	Init(trader *Trader) error
	OnNewStream(stream *binance.PrivateStream) error
}

type RegressionTrader struct {
	// Context is trading Context
	Context *TradingContext

}

func (trader *RegressionTrader) RunStrategy(ctx context.Context, strategy Strategy) {


}



type Trader struct {
	Notifier *SlackNotifier

	// Context is trading Context
	Context *TradingContext

	Exchange *binance.Exchange

	reportTimer *time.Timer
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

	if err := strategy.Init(trader) ; err != nil {
		return nil, err
	}

	stream, err := trader.Exchange.NewPrivateStream()
	if err != nil {
		return nil, err
	}

	if err := strategy.OnNewStream(stream); err != nil {
		return nil, err
	}

	trader.reportTimer = time.AfterFunc(1*time.Second, func() {
		trader.ReportPnL()
	})

	stream.OnTrade(func(trade *types.Trade) {
		if trade.Symbol != symbol {
			return
		}

		trader.ReportTrade(trade)
		trader.Context.ProfitAndLossCalculator.AddTrade(*trade)

		if trader.reportTimer != nil {
			trader.reportTimer.Stop()
		}

		trader.reportTimer = time.AfterFunc(5*time.Second, func() {
			trader.ReportPnL()
		})
	})

	stream.OnKLineEvent(func(e *binance.KLineEvent) {
		trader.Context.SetCurrentPrice(e.KLine.GetClose())
	})

	stream.OnOutboundAccountInfoEvent(func(e *binance.OutboundAccountInfoEvent) {
		trader.Context.Lock()
		defer trader.Context.Unlock()

		for _, balance := range e.Balances {
			available := util.MustParseFloat(balance.Free)
			locked := util.MustParseFloat(balance.Locked)
			trader.Context.Balances[balance.Asset] = types.Balance{
				Currency:  balance.Asset,
				Available: available,
				Locked:    locked,
			}
		}
	})

	stream.OnBalanceUpdateEvent(func(e *binance.BalanceUpdateEvent) {
		trader.Context.Lock()
		defer trader.Context.Unlock()

		delta := util.MustParseFloat(e.Delta)
		if balance, ok := trader.Context.Balances[e.Asset] ; ok {
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
	report := trader.Context.ProfitAndLossCalculator.Calculate()
	report.Print()
	trader.Notifier.ReportPnL(report)
}

func (trader *Trader) SubmitOrder(ctx context.Context, order *types.Order) {
	trader.Notifier.Notify(":memo: Submitting %s order on side %s with volume: %s", order.Type, order.Side, order.VolumeStr, order.SlackAttachment())

	err := trader.Exchange.SubmitOrder(ctx, order)
	if err != nil {
		log.WithError(err).Errorf("order create error: side %s volume: %s", order.Side, order.VolumeStr)
		return
	}
}
