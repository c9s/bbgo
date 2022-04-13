package ewo_dgtrd

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "ewo_dgtrd"

var log = logrus.WithField("strategy", ID)
var modifier = fixedpoint.NewFromFloat(0.99)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	*bbgo.Graceful
	bbgo.SmartStops
	tradeCollector *bbgo.TradeCollector
	Symbol         string         `json:"symbol"`
	Interval       types.Interval `json:"interval"`
	UseEma         bool           `json:"useEma"` // use exponential ma or simple ma
	SignalWindow   int            `json:"sigWin"` // signal window
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Initialize() error {
	return s.SmartStops.InitializeStopControllers(s.Symbol)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	log.Infof("subscribe %s", s.Symbol)
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval.String()})
	s.SmartStops.Subscribe(session)
}

type EwoSignal interface {
	types.Series
	Update(value float64)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	market, ok := session.Market(s.Symbol)
	if !ok {
		log.Errorf("fetch market fail %s", s.Symbol)
		return nil
	}

	indicatorSet, ok := session.StandardIndicatorSet(s.Symbol)
	if !ok {
		log.Errorf("cannot get indicatorSet of %s", s.Symbol)
		return nil
	}
	orderbook, ok := session.OrderStore(s.Symbol)
	if !ok {
		log.Errorf("cannot get orderbook of %s", s.Symbol)
		return nil
	}
	position := types.NewPositionFromMarket(market)
	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, position, orderbook)
	s.tradeCollector.OnTrade(func(trade types.Trade, profit, netprofit fixedpoint.Value) {
		if !profit.IsZero() {
			log.Warnf("generate profit: %v, netprofit: %v", profit, netprofit)
		}
	})

	s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
		log.Infof("position changed: %s", position)
	})
	s.tradeCollector.BindStream(session.UserDataStream)

	s.SmartStops.RunStopControllers(ctx, session, s.tradeCollector)

	/*store, ok := session.MarketDataStore(s.Symbol)
	if !ok {
		log.Errorf("cannot get marketdatastore of %s", s.Symbol)
		return nil
	}*/

	/*window, ok := store.KLinesOfInterval(s.Interval)
	if !ok {
		log.Errorf("cannot get klinewindow of %s", s.Interval)
	}*/
	var ma5, ma34, ma50, ewo types.Series
	if s.UseEma {
		ma5 = indicatorSet.EWMA(types.IntervalWindow{s.Interval, 5})
		ma34 = indicatorSet.EWMA(types.IntervalWindow{s.Interval, 34})
		ma50 = indicatorSet.EWMA(types.IntervalWindow{s.Interval, 50})
	} else {
		ma5 = indicatorSet.SMA(types.IntervalWindow{s.Interval, 5})
		ma34 = indicatorSet.SMA(types.IntervalWindow{s.Interval, 34})
		ma50 = indicatorSet.SMA(types.IntervalWindow{s.Interval, 50})
	}
	ewo = types.Mul(types.Minus(types.Div(ma5, ma34), 1.0), 100.)
	var ewoSignal EwoSignal
	if s.UseEma {
		ewoSignal = &indicator.EWMA{IntervalWindow: types.IntervalWindow{s.Interval, s.SignalWindow}}
	} else {
		ewoSignal = &indicator.SMA{IntervalWindow: types.IntervalWindow{s.Interval, s.SignalWindow}}
	}
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != s.Symbol {
			return
		}

		if kline.Interval == s.Interval {
			if ewoSignal.Length() == 0 {
				// lazy init
				ewoVals := types.ToReverseArray(ewo)
				for _, ewoValue := range ewoVals {
					ewoSignal.Update(ewoValue)
				}
			} else {
				ewoSignal.Update(ewo.Last())
			}
		}

		/*lastPrice, ok := session.LastPrice(s.Symbol)
		if !ok {
			log.Errorf("cannot get last price")
			return
		}*/

		// cancel non-traded orders
		var toCancel []types.Order
		var toRepost []types.SubmitOrder
		for _, order := range orderbook.Orders() {
			if order.Status == types.OrderStatusNew || order.Status == types.OrderStatusPartiallyFilled {
				toCancel = append(toCancel, order)
			}
		}
		if len(toCancel) > 0 {
			if err := orderExecutor.CancelOrders(ctx, toCancel...); err != nil {
				log.WithError(err).Errorf("cancel order error")
			}

			s.tradeCollector.Process()
		}

		// well, only track prices on 1m
		if kline.Interval != s.Interval {
			for _, order := range toCancel {
				if order.Side == types.SideTypeBuy && order.Price.Compare(kline.Low) < 0 {
					order.Quantity = order.Quantity.Mul(order.Price).Div(kline.Low)
					order.Price = kline.Low
					toRepost = append(toRepost, order.SubmitOrder)
				} else if order.Side == types.SideTypeSell && order.Price.Compare(kline.High) > 0 {
					order.Price = kline.High
					toRepost = append(toRepost, order.SubmitOrder)
				}
			}

			if len(toRepost) > 0 {
				createdOrders, err := orderExecutor.SubmitOrders(ctx, toRepost...)
				if err != nil {
					log.WithError(err).Errorf("cannot place order")
					return
				}
				log.Infof("repost order %v", createdOrders)
				s.tradeCollector.Process()
			}
		}

		if kline.Interval != s.Interval {
			return
		}

		// To get the threshold for ewo
		mean := types.Mean(types.Abs(ewo), 7)

		longSignal := types.CrossOver(ewo, ewoSignal)
		shortSignal := types.CrossUnder(ewo, ewoSignal)
		bull := types.Predict(ma50, 50, 2) > ma50.Last()
		// kline breakthrough ma5, ma50 trend up, and ewo > threshold
		IsBull := bull && kline.High.Float64() > ma5.Last() && ewo.Last() > mean
		// kline downthrough ma5, ma50 trend down, and ewo < threshold
		IsBear := !bull && kline.Low.Float64() < ma5.Last() && ewo.Last() < -mean

		var orders []types.SubmitOrder

		if longSignal.Index(1) && !shortSignal.Last() && IsBull {
			price := kline.Low
			quoteBalance, ok := session.Account.Balance(market.QuoteCurrency)
			if !ok {
				return
			}
			quantityAmount := quoteBalance.Available.Mul(modifier)
			totalQuantity := quantityAmount.Div(price)
			if quantityAmount.Sign() <= 0 ||
				quantityAmount.Compare(market.MinNotional) < 0 ||
				totalQuantity.Compare(market.MinQuantity) < 0 {
				log.Infof("quote balance %v is not enough. stop generating buy orders", quoteBalance)
				return
			}
			// strong long
			log.Infof("long at %v, timestamp: %s", price, kline.StartTime)

			orders = append(orders, types.SubmitOrder{
				Symbol:      kline.Symbol,
				Side:        types.SideTypeBuy,
				Type:        types.OrderTypeLimit,
				Price:       price,
				Quantity:    totalQuantity,
				Market:      market,
				TimeInForce: types.TimeInForceGTC,
			})
		} else if shortSignal.Index(1) && !longSignal.Last() && IsBear {
			price := kline.High
			balances := session.Account.Balances()
			baseBalance := balances[market.BaseCurrency].Available.Mul(modifier)
			baseAmount := baseBalance.Mul(price)
			if baseBalance.Sign() <= 0 ||
				baseBalance.Compare(market.MinQuantity) < 0 ||
				baseAmount.Compare(market.MinNotional) < 0 {
				log.Infof("base balance %v is not enough. stop generating sell orders", baseBalance)
				return
			}
			log.Infof("short at %v, timestamp: %s", price, kline.StartTime)
			orders = append(orders, types.SubmitOrder{
				Symbol:      s.Symbol,
				Side:        types.SideTypeSell,
				Type:        types.OrderTypeLimit,
				Market:      market,
				Quantity:    baseBalance,
				Price:       price,
				TimeInForce: types.TimeInForceGTC,
			})
		}
		if len(orders) > 0 {
			createdOrders, err := orderExecutor.SubmitOrders(ctx, orders...)
			if err != nil {
				log.WithError(err).Errorf("cannot place order")
				return
			}
			log.Infof("post order %v", createdOrders)
			s.tradeCollector.Process()
		}
	})
	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		log.Infof("canceling active orders...")

		var toCancel []types.Order
		for _, order := range orderbook.Orders() {
			if order.Status == types.OrderStatusNew || order.Status == types.OrderStatusPartiallyFilled {
				toCancel = append(toCancel, order)
			}
		}

		if err := orderExecutor.CancelOrders(ctx, toCancel...); err != nil {
			log.WithError(err).Errorf("cancel order error")
		}
		s.tradeCollector.Process()
	})
	return nil
}
