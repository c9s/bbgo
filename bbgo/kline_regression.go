package bbgo

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo/accounting"
	"github.com/c9s/bbgo/pkg/bbgo/types"
	"github.com/c9s/bbgo/pkg/util"
)

type BackTestStream struct {
	types.StandardPrivateStream
}


func (s *BackTestStream) Connect(ctx context.Context) error {
	return nil
}

func (s *BackTestStream) Close() error {
	return nil
}

type BackTestTrader struct {
	// Context is trading Context
	Context                 *Context
	SourceKLines            []types.KLine
	ProfitAndLossCalculator *accounting.ProfitAndLossCalculator

	doneOrders    []*types.SubmitOrder
	pendingOrders []*types.SubmitOrder
}

func (trader *BackTestTrader) SubmitOrder(cxt context.Context, order *types.SubmitOrder) {
	trader.pendingOrders = append(trader.pendingOrders, order)
}

func (trader *BackTestTrader) RunStrategy(ctx context.Context, strategy Strategy) (chan struct{}, error) {
	logrus.Infof("[regression] number of kline data: %d", len(trader.SourceKLines))

	maxExposure := 0.4
	trader.Context.Quota = make(map[string]types.Balance)
	for currency, balance := range trader.Context.Balances {
		quota := balance
		quota.Available *= maxExposure
		trader.Context.Quota[currency] = quota
	}

	done := make(chan struct{})
	defer close(done)

	if err := strategy.Load(trader.Context, trader); err != nil {
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
			trader.ProfitAndLossCalculator.AddTrade(trade)

			trader.doneOrders = append(trader.doneOrders, order)
		}

		// clear pending orders
		trader.pendingOrders = nil
	}

	fmt.Print("\n")
	report := trader.ProfitAndLossCalculator.Calculate()
	report.Print()

	logrus.Infof("wallet balance:")
	for _, balance := range trader.Context.Balances {
		logrus.Infof(" %s: %f", balance.Currency, balance.Available)
	}

	return done, nil
}
