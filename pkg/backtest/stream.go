package backtest

import (
	"context"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

type Stream struct {
	types.StandardStream

	exchange *Exchange
}

func (s *Stream) Connect(ctx context.Context) error {
	log.Infof("collecting backtest configurations...")

	loadedSymbols := map[string]struct{}{}
	loadedIntervals := map[types.Interval]struct{}{
		// 1m interval is required for the backtest matching engine
		types.Interval1m: struct{}{},
	}

	for _, sub := range s.Subscriptions {
		loadedSymbols[sub.Symbol] = struct{}{}

		switch sub.Channel {
		case types.KLineChannel:
			loadedIntervals[types.Interval(sub.Options.Interval)] = struct{}{}

		default:
			return errors.Errorf("stream channel %s is not supported in backtest", sub.Channel)
		}
	}

	var symbols []string
	for symbol := range loadedSymbols {
		symbols = append(symbols, symbol)
	}

	var intervals []types.Interval
	for interval := range loadedIntervals {
		intervals = append(intervals, interval)
	}

	log.Infof("used symbols: %v and intervals: %v", symbols, intervals)

	go func() {
		klineC, errC := s.exchange.srv.QueryKLinesCh(s.exchange.startTime, s.exchange, symbols, intervals)
		for k := range klineC {
			if k.Interval == types.Interval1m {
				matching, ok := s.exchange.matchingBooks[k.Symbol]
				if !ok {
					log.Error("matching book of %s is not initialized", k.Symbol)
				}
				matching.processKLine(s, k)
			}

			s.EmitKLineClosed(k)
		}

		if err := <-errC; err != nil {
			log.WithError(err).Error("backtest data feed error")
		}

		if err := s.Close(); err != nil {
			log.WithError(err).Error("stream close error")
		}
	}()

	return nil
}

func (s *Stream) Close() error {
	close(s.exchange.doneC)
	return nil
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
