package bbgo

import (
	"context"
	"fmt"
	"github.com/c9s/bbgo/pkg/util"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo/exchange/binance"
	"github.com/c9s/bbgo/pkg/bbgo/types"
)

type Strategy interface {
	Init(tradingContext *TradingContext, trader types.Trader) error
	OnNewStream(stream *types.StandardPrivateStream) error
}

type KLineRegressionTrader struct {
	// Context is trading Context
	Context                 *TradingContext
	SourceKLines            []types.KLine
	ProfitAndLossCalculator *ProfitAndLossCalculator

	orderC chan *types.Order
}

func (trader *KLineRegressionTrader) SubmitOrder(cxt context.Context, order *types.Order) {
	trader.orderC <- order
}

func (trader *KLineRegressionTrader) RunStrategy(ctx context.Context, strategy Strategy) (chan struct{}, error) {
	log.Infof("[regression] number of kline data: %d", len(trader.SourceKLines))

	trader.orderC = make(chan *types.Order, 12)

	done := make(chan struct{})
	defer close(done)

	if err := strategy.Init(trader.Context, trader); err != nil {
		return nil, err
	}

	standardStream := types.StandardPrivateStream{}
	if err := strategy.OnNewStream(&standardStream); err != nil {
		return nil, err
	}

	var tradeID int64 = 0
	for _, kline := range trader.SourceKLines {
		log.Debugf("kline %+v", kline)

		fmt.Print(".")

		standardStream.EmitKLineClosed(&kline)

		select {

		case order := <-trader.orderC:
			fmt.Print("!")

			var price float64
			if order.Type == types.OrderTypeLimit {
				price = util.MustParseFloat(order.PriceStr)
			} else {
				price = kline.GetClose()
			}

			volume := util.MustParseFloat(order.VolumeStr)
			fee := 0.0
			feeCurrency := ""

			if order.Side == types.SideTypeBuy {
				fee = price * volume * 0.001
				feeCurrency = "USDT"

				quote := trader.Context.Balances[trader.Context.Market.QuoteCurrency]
				quote.Available -= volume * price
				trader.Context.Balances[trader.Context.Market.QuoteCurrency] = quote

				base := trader.Context.Balances[trader.Context.Market.BaseCurrency]
				base.Available += volume
				trader.Context.Balances[trader.Context.Market.BaseCurrency] = base

			} else {
				fee = volume * 0.001
				feeCurrency = "BTC"

				quote := trader.Context.Balances[trader.Context.Market.QuoteCurrency]
				quote.Available += volume * price
				trader.Context.Balances[trader.Context.Market.QuoteCurrency] = quote

				base := trader.Context.Balances[trader.Context.Market.BaseCurrency]
				base.Available -= volume
				trader.Context.Balances[trader.Context.Market.BaseCurrency] = base
			}

			trade := types.Trade{
				ID:          tradeID,
				Price:       price,
				Volume:      volume,
				Side:        string(order.Side),
				IsBuyer:     order.Side == types.SideTypeBuy,
				IsMaker:     false,
				Time:        time.Unix(0, kline.EndTime*int64(time.Millisecond)),
				Symbol:      trader.Context.Symbol,
				Fee:         fee,
				FeeCurrency: feeCurrency,
			}

			tradeID++
			trader.ProfitAndLossCalculator.AddTrade(trade)
		default:
		}
	}

	fmt.Print("\n")
	report := trader.ProfitAndLossCalculator.Calculate()
	report.Print()

	return done, nil
}

type Trader struct {
	Notifier *SlackNotifier

	// Context is trading Context
	Context *TradingContext

	Exchange *binance.Exchange

	reportTimer *time.Timer

	ProfitAndLossCalculator *ProfitAndLossCalculator
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

		trader.ReportTrade(trade)
		trader.ProfitAndLossCalculator.AddTrade(*trade)

		if trader.reportTimer != nil {
			trader.reportTimer.Stop()
		}

		trader.reportTimer = time.AfterFunc(5*time.Second, func() {
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

func (trader *Trader) SubmitOrder(ctx context.Context, order *types.Order) {
	trader.Notifier.Notify(":memo: Submitting %s order on side %s with volume: %s", order.Type, order.Side, order.VolumeStr, order.SlackAttachment())

	err := trader.Exchange.SubmitOrder(ctx, order)
	if err != nil {
		log.WithError(err).Errorf("order create error: side %s volume: %s", order.Side, order.VolumeStr)
		return
	}
}
