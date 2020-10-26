package backtest

import (
	"context"

	"github.com/c9s/bbgo/pkg/accounting/pnl"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

type Stream struct {
	types.StandardStream
}

func (s *Stream) Connect(ctx context.Context) error {
	return nil
}

func (s *Stream) Close() error {
	return nil
}

type Trader struct {
	// Context is trading Context
	Context                 *bbgo.Context
	SourceKLines            []types.KLine
	ProfitAndLossCalculator *pnl.AverageCostCalculator

	doneOrders    []types.SubmitOrder
	pendingOrders []types.SubmitOrder
}

/*
func (trader *BackTestTrader) RunStrategy(ctx context.Context, strategy SingleExchangeStrategy) (chan struct{}, error) {
	logrus.Infof("[regression] number of kline data: %d", len(trader.SourceKLines))

	done := make(chan struct{})
	defer close(done)

	if err := strategy.OnLoad(trader.Context, trader); err != nil {
		return nil, err
	}

	stream := &BackTestStream{}
	if err := strategy.OnNewStream(stream); err != nil {
		return nil, err
	}

	var tradeID int64 = 0
	for _, kline := range trader.SourceKLines {
		logrus.Debugf("kline %+v", kline)

		fmt.Print(".")

		stream.EmitKLineClosed(kline)

		for _, order := range trader.pendingOrders {
			switch order.Side {
			case types.SideTypeBuy:
				fmt.Print("B")
			case types.SideTypeSell:
				fmt.Print("S")
			}

			var price float64
			if order.Type == types.OrderTypeLimit {
				price = util.MustParseFloat(order.PriceString)
			} else {
				price = kline.GetClose()
			}

			volume := util.MustParseFloat(order.QuantityString)
			fee := 0.0
			feeCurrency := ""

			trader.Context.Lock()
			if order.Side == types.SideTypeBuy {
				fee = price * volume * 0.001
				feeCurrency = "USDT"

				quote := trader.Context.Balances[trader.Context.Market.QuoteCurrency]

				if quote.Available < volume*price {
					logrus.Fatalf("quote balance not enough: %+v", quote)
				}
				quote.Available -= volume * price
				trader.Context.Balances[trader.Context.Market.QuoteCurrency] = quote

				base := trader.Context.Balances[trader.Context.Market.BaseCurrency]
				base.Available += volume
				trader.Context.Balances[trader.Context.Market.BaseCurrency] = base

			} else {
				fee = volume * 0.001
				feeCurrency = "BTC"

				base := trader.Context.Balances[trader.Context.Market.BaseCurrency]
				if base.Available < volume {
					logrus.Fatalf("base balance not enough: %+v", base)
				}

				base.Available -= volume
				trader.Context.Balances[trader.Context.Market.BaseCurrency] = base

				quote := trader.Context.Balances[trader.Context.Market.QuoteCurrency]
				quote.Available += volume * price
				trader.Context.Balances[trader.Context.Market.QuoteCurrency] = quote
			}
			trader.Context.Unlock()

			trade := types.Trade{
				ID:          tradeID,
				Price:       price,
				Quantity:    volume,
				Side:        string(order.Side),
				IsBuyer:     order.Side == types.SideTypeBuy,
				IsMaker:     false,
				Time:        kline.EndTime,
				Symbol:      trader.Context.Symbol,
				Fee:         fee,
				FeeCurrency: feeCurrency,
			}

			tradeID++
			trader.AverageCostCalculator.AddTrade(trade)

			trader.doneOrders = append(trader.doneOrders, order)
		}

		// clear pending orders
		trader.pendingOrders = nil
	}

	fmt.Print("\n")
	report := trader.AverageCostCalculator.Calculate()
	report.Print()

	logrus.Infof("wallet balance:")
	for _, balance := range trader.Context.Balances {
		logrus.Infof(" %s: %f", balance.Currency, balance.Available)
	}

	return done, nil
}
*/
