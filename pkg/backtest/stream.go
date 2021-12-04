package backtest

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

var log = logrus.WithField("cmd", "backtest")

type Stream struct {
	types.StandardStream

	exchange *Exchange

	publicOnly bool
}

func (s *Stream) Connect(ctx context.Context) error {
	log.Infof("collecting backtest configurations...")

	loadedSymbols := map[string]struct{}{}
	loadedIntervals := map[types.Interval]struct{}{
		// 1m interval is required for the backtest matching engine
		types.Interval1m: {},
		types.Interval1d: {},
	}

	for _, sub := range s.Subscriptions {
		loadedSymbols[sub.Symbol] = struct{}{}

		switch sub.Channel {
		case types.KLineChannel:
			loadedIntervals[types.Interval(sub.Options.Interval)] = struct{}{}

		default:
			return fmt.Errorf("stream channel %s is not supported in backtest", sub.Channel)
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

	if !s.publicOnly {
		// user data stream
		s.OnTradeUpdate(func(trade types.Trade) {
			s.exchange.trades[trade.Symbol] = append(s.exchange.trades[trade.Symbol], trade)
		})

		for symbol, market := range s.exchange.markets {
			matching := &SimplePriceMatching{
				CurrentTime:     s.exchange.startTime,
				Account:         s.exchange.account,
				Market:          market,
				MakerCommission: s.exchange.config.Account.MakerCommission,
				TakerCommission: s.exchange.config.Account.TakerCommission,
			}
			matching.OnTradeUpdate(s.EmitTradeUpdate)
			matching.OnOrderUpdate(s.EmitOrderUpdate)
			matching.OnBalanceUpdate(s.EmitBalanceUpdate)
			s.exchange.matchingBooks[symbol] = matching
		}

		// assign user data stream back
		s.exchange.userDataStream = s
	}

	s.EmitConnect()
	s.EmitStart()

	if s.publicOnly {
		go func() {
			log.Infof("querying klines from database...")
			klineC, errC := s.exchange.srv.QueryKLinesCh(s.exchange.startTime, s.exchange.endTime, s.exchange, symbols, intervals)
			numKlines := 0
			for k := range klineC {
				if k.Interval == types.Interval1m {
					matching, ok := s.exchange.matchingBook(k.Symbol)
					if !ok {
						log.Errorf("matching book of %s is not initialized", k.Symbol)
						continue
					}

					// here we generate trades and order updates
					matching.processKLine(k)
					numKlines++
				}

				s.EmitKLineClosed(k)
			}

			if err := <-errC; err != nil {
				log.WithError(err).Error("backtest data feed error")
			}

			if numKlines == 0 {
				log.Error("kline data is empty, make sure you have sync the exchange market data")
			}

			if err := s.Close(); err != nil {
				log.WithError(err).Error("stream close error")
			}
		}()
	}

	return nil
}

func (s *Stream) SetPublicOnly() {
	s.publicOnly = true
	return
}

func (s *Stream) Close() error {
	close(s.exchange.doneC)
	return nil
}
