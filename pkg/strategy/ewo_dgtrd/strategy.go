package ewo_dgtrd

import (
	"context"

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
	Symbol       string           `json:"symbol"`
	Interval     types.Interval   `json:"interval"`
	Threshold    float64          `json:"threshold"` // strength threshold
	UseEma       bool             `json:"useEma"`    // use exponential ma or simple ma
	SignalWindow int              `json:"sigWin"`    // signal window
	StopLoss     fixedpoint.Value `json:"stoploss"`  // stop price = latest price * (1 - stoploss)
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	log.Infof("subscribe %s", s.Symbol)
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval.String()})
}

type EwoSignal interface {
	types.Series
	Update(value float64)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	log.Infof("stoploss: %v", s.StopLoss)
	indicatorSet, ok := session.StandardIndicatorSet(s.Symbol)
	if !ok {
		log.Errorf("cannot get indicatorSet of %s", s.Symbol)
		return nil
	}
	orders, ok := session.OrderStore(s.Symbol)
	if !ok {
		log.Errorf("cannot get orderbook of %s", s.Symbol)
		return nil
	}
	/*store, ok := session.MarketDataStore(s.Symbol)
	if !ok {
		log.Errorf("cannot get marketdatastore of %s", s.Symbol)
		return nil
	}*/
	market, ok := session.Market(s.Symbol)
	if !ok {
		log.Errorf("fetch market fail %s", s.Symbol)
		return nil
	}
	/*window, ok := store.KLinesOfInterval(s.Interval)
	if !ok {
		log.Errorf("cannot get klinewindow of %s", s.Interval)
	}*/
	var ma5, ma34, ewo types.Series
	if s.UseEma {
		ma5 = indicatorSet.EWMA(types.IntervalWindow{s.Interval, 5})
		ma34 = indicatorSet.EWMA(types.IntervalWindow{s.Interval, 34})
	} else {
		ma5 = indicatorSet.SMA(types.IntervalWindow{s.Interval, 5})
		ma34 = indicatorSet.SMA(types.IntervalWindow{s.Interval, 34})
	}
	ewo = types.Mul(types.Minus(types.Div(ma5, ma34), 1.0), 100.)
	var ewoSignal EwoSignal
	if s.UseEma {
		ewoSignal = &indicator.EWMA{IntervalWindow: types.IntervalWindow{s.Interval, s.SignalWindow}}
	} else {
		ewoSignal = &indicator.SMA{IntervalWindow: types.IntervalWindow{s.Interval, s.SignalWindow}}
	}
	entryPrice := fixedpoint.Zero
	stopPrice := fixedpoint.Zero
	tradeDirectionLong := true
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

		lastPrice, ok := session.LastPrice(s.Symbol)
		if !ok {
			return
		}

		// stoploss
		if tradeDirectionLong && kline.Low.Compare(stopPrice) <= 0 && !stopPrice.IsZero() {
			balances := session.Account.Balances()
			baseBalance := balances[market.BaseCurrency].Available.Mul(modifier)
			baseAmount := baseBalance.Mul(lastPrice)
			if baseBalance.Sign() <= 0 ||
				baseBalance.Compare(market.MinQuantity) < 0 ||
				baseAmount.Compare(market.MinNotional) < 0 {
			} else {
				_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
					Symbol:      kline.Symbol,
					Side:        types.SideTypeSell,
					Type:        types.OrderTypeMarket,
					Quantity:    baseBalance,
					Price:       lastPrice,
					Market:      market,
					TimeInForce: types.TimeInForceGTC,
				})
				if err != nil {
					log.WithError(err).Errorf("cannot place order for stoploss")
					return
				}
				log.Warnf("StopLoss Long at %v", lastPrice)
				entryPrice = fixedpoint.Zero
				stopPrice = fixedpoint.Zero
			}
		} else if !tradeDirectionLong && kline.High.Compare(stopPrice) >= 0 && !stopPrice.IsZero() {
			quoteBalance, ok := session.Account.Balance(market.QuoteCurrency)
			if !ok {
				return
			}
			quantityAmount := quoteBalance.Available
			totalQuantity := quantityAmount.Div(lastPrice).Mul(modifier)
			if quantityAmount.Sign() <= 0 ||
				quantityAmount.Compare(market.MinNotional) < 0 ||
				totalQuantity.Compare(market.MinQuantity) < 0 {
			} else {
				_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
					Symbol:      kline.Symbol,
					Side:        types.SideTypeBuy,
					Type:        types.OrderTypeMarket,
					Quantity:    totalQuantity,
					Price:       lastPrice,
					Market:      market,
					TimeInForce: types.TimeInForceGTC,
				})
				if err != nil {
					log.WithError(err).Errorf("cannot place order for stoploss")
					return
				}
				log.Warnf("StopLoss Short at %v", lastPrice)
				entryPrice = fixedpoint.Zero
				stopPrice = fixedpoint.Zero
			}
		}

		if kline.Interval != s.Interval {
			return
		}
		var toCancel []types.Order
		for _, order := range orders.Orders() {
			if order.Status == types.OrderStatusNew || order.Status == types.OrderStatusPartiallyFilled {
				toCancel = append(toCancel, order)
			}
		}
		if err := orderExecutor.CancelOrders(ctx, toCancel...); err != nil {
			log.WithError(err).Errorf("cancel order error")
		}

		longSignal := types.CrossOver(ewo, ewoSignal)
		shortSignal := types.CrossUnder(ewo, ewoSignal)
		IsBull := kline.Close.Compare(kline.Open) > 0

		var orders []types.SubmitOrder
		price := lastPrice
		if longSignal.Index(1) && !shortSignal.Last() && IsBull {
			quoteBalance, ok := session.Account.Balance(market.QuoteCurrency)
			if !ok {
				return
			}
			quantityAmount := quoteBalance.Available
			totalQuantity := quantityAmount.Div(price).Mul(modifier).Div(types.Two)
			if quantityAmount.Sign() <= 0 ||
				quantityAmount.Compare(market.MinNotional) < 0 ||
				totalQuantity.Compare(market.MinQuantity) < 0 {
				log.Infof("quote balance %v is not enough. stop generating buy orders", quoteBalance)
				return
			}
			if ewo.Last() < -s.Threshold {
				// strong long
				log.Infof("strong long at %v, timestamp: %s", price, kline.StartTime)

				orders = append(orders, types.SubmitOrder{
					Symbol:      kline.Symbol,
					Side:        types.SideTypeBuy,
					Type:        types.OrderTypeMarket,
					Price:       price,
					Quantity:    totalQuantity.Mul(types.Two),
					Market:      market,
					TimeInForce: types.TimeInForceGTC,
				})
			} else if ewo.Last() < 0 {
				log.Infof("long at %v, timestamp: %s", price, kline.StartTime)
				// Long

				// TODO: smaller quantity?

				orders = append(orders, types.SubmitOrder{
					Symbol:      s.Symbol,
					Side:        types.SideTypeBuy,
					Type:        types.OrderTypeMarket,
					Price:       price,
					Quantity:    totalQuantity,
					Market:      market,
					TimeInForce: types.TimeInForceGTC,
				})
			}
		} else if shortSignal.Index(1) && !longSignal.Last() && !IsBull {
			balances := session.Account.Balances()
			baseBalance := balances[market.BaseCurrency].Available.Mul(modifier).Div(types.Two)
			baseAmount := baseBalance.Mul(price)
			if baseBalance.Sign() <= 0 ||
				baseBalance.Compare(market.MinQuantity) < 0 ||
				baseAmount.Compare(market.MinNotional) < 0 {
				log.Infof("base balance %v is not enough. stop generating sell orders", baseBalance)
				return
			}
			if ewo.Last() > s.Threshold {
				// Strong short
				log.Infof("strong short at %v, timestamp: %s", price, kline.StartTime)
				orders = append(orders, types.SubmitOrder{
					Symbol:      s.Symbol,
					Side:        types.SideTypeSell,
					Type:        types.OrderTypeMarket,
					Market:      market,
					Quantity:    baseBalance.Mul(types.Two),
					Price:       price,
					TimeInForce: types.TimeInForceGTC,
				})
			} else if ewo.Last() > 0 {
				// short
				log.Infof("short at %v, timestamp: %s", price, kline.StartTime)
				// TODO: smaller quantity?
				orders = append(orders, types.SubmitOrder{
					Symbol:      s.Symbol,
					Side:        types.SideTypeSell,
					Type:        types.OrderTypeMarket,
					Market:      market,
					Quantity:    baseBalance,
					Price:       price,
					TimeInForce: types.TimeInForceGTC,
				})
			}
		}
		if len(orders) > 0 {
			createdOrders, err := orderExecutor.SubmitOrders(ctx, orders...)
			if err != nil {
				log.WithError(err).Errorf("cannot place order")
				return
			}
			entryPrice = lastPrice
			tradeDirectionLong = IsBull
			if tradeDirectionLong {
				stopPrice = entryPrice.Mul(fixedpoint.One.Sub(s.StopLoss))
			} else {
				stopPrice = entryPrice.Mul(fixedpoint.One.Add(s.StopLoss))
			}
			log.Infof("Place orders %v stop @ %v", createdOrders, stopPrice)
		}
	})
	return nil
}
