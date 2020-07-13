package bbgo

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo/exchange/binance"
	"github.com/c9s/bbgo/pkg/bbgo/types"
)

type Trader struct {
	Notifier *SlackNotifier

	// Context is trading Context
	Context *TradingContext

	Exchange *binance.Exchange

	reportTimer *time.Timer
}


type Strategy interface {
	Init(trader *Trader, stream *binance.PrivateStream) error
}

func (t *Trader) RunStrategy(ctx context.Context, strategy Strategy) error {
	symbol := t.Context.Symbol

	stream, err := t.Exchange.NewPrivateStream()
	if err != nil {
		return err
	}

	if err := strategy.Init(t, stream); err != nil {
		return err
	}

	t.reportTimer = time.AfterFunc(1*time.Second, func() {
		t.ReportPnL()
	})

	stream.OnTrade(func(trade *types.Trade) {
		if trade.Symbol != symbol {
			return
		}

		t.ReportTrade(trade)
		t.Context.ProfitAndLossCalculator.AddTrade(*trade)

		if t.reportTimer != nil {
			t.reportTimer.Stop()
		}

		t.reportTimer = time.AfterFunc(5*time.Second, func() {
			t.ReportPnL()
		})
	})

	stream.OnKLineEvent(func(e *binance.KLineEvent) {
		t.Context.SetCurrentPrice(e.KLine.GetClose())
	})

	var eventC = make(chan interface{}, 20)
	if err := stream.Connect(ctx, eventC); err != nil {
		return err
	}

	go func() {
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

	return nil
}

func (t *Trader) Infof(format string, args ...interface{}) {
	t.Notifier.Notify(format, args...)
}

func (t *Trader) ReportTrade(trade *types.Trade) {
	t.Notifier.ReportTrade(trade)
}

func (t *Trader) ReportPnL() {
	report := t.Context.ProfitAndLossCalculator.Calculate()
	report.Print()
	t.Notifier.ReportPnL(report)
}

func (t *Trader) SubmitOrder(ctx context.Context, order *types.Order) {
	t.Notifier.Notify(":memo: Submitting %s order on side %s with volume: %s", order.Type, order.Side, order.VolumeStr, order.SlackAttachment())

	err := t.Exchange.SubmitOrder(ctx, order)
	if err != nil {
		log.WithError(err).Errorf("order create error: side %s volume: %s", order.Side, order.VolumeStr)
		return
	}
}
