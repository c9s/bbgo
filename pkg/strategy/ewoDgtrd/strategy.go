package ewoDgtrd

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "ewo_dgtrd"

var log = logrus.WithField("strategy", ID)
var modifier = fixedpoint.NewFromFloat(0.995)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Market        types.Market
	Session       *bbgo.ExchangeSession
	UseHeikinAshi bool             `json:"useHeikinAshi"` // use heikinashi kline
	Stoploss      fixedpoint.Value `json:"stoploss"`
	Callback      fixedpoint.Value `json:"callback"`
	Symbol        string           `json:"symbol"`
	Interval      types.Interval   `json:"interval"`
	UseEma        bool             `json:"useEma"` // use exponential ma or not
	UseSma        bool             `json:"useSma"` // if UseEma == false, use simple ma or not
	SignalWindow  int              `json:"sigWin"` // signal window

	*bbgo.Graceful
	bbgo.SmartStops
	tradeCollector *bbgo.TradeCollector
	ma5            types.Series
	ma34           types.Series
	ewo            types.Series
	ewoSignal      types.Series
	heikinAshi     *HeikinAshi
	peakPrice      fixedpoint.Value
	bottomPrice    fixedpoint.Value
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

type UpdatableSeries interface {
	types.Series
	Update(value float64)
}

type VWEMA struct {
	PV UpdatableSeries
	V  UpdatableSeries
}

func (inc *VWEMA) Last() float64 {
	return inc.PV.Last() / inc.V.Last()
}

func (inc *VWEMA) Index(i int) float64 {
	if i >= inc.PV.Length() {
		return 0
	}
	vi := inc.V.Index(i)
	if vi == 0 {
		return 0
	}
	return inc.PV.Index(i) / vi
}

func (inc *VWEMA) Length() int {
	pvl := inc.PV.Length()
	vl := inc.V.Length()
	if pvl < vl {
		return pvl
	}
	return vl
}

func (inc *VWEMA) Update(kline types.KLine) {
	inc.PV.Update(kline.Close.Mul(kline.Volume).Float64())
	inc.V.Update(kline.Volume.Float64())
}

func (inc *VWEMA) UpdateVal(price float64, vol float64) {
	inc.PV.Update(price * vol)
	inc.V.Update(vol)
}

type Queue struct {
	arr  []float64
	size int
}

func NewQueue(size int) *Queue {
	return &Queue{
		arr:  make([]float64, 0, size),
		size: size,
	}
}

func (inc *Queue) Last() float64 {
	if len(inc.arr) == 0 {
		return 0
	}
	return inc.arr[len(inc.arr)-1]
}

func (inc *Queue) Index(i int) float64 {
	if len(inc.arr)-i-1 < 0 {
		return 0
	}
	return inc.arr[len(inc.arr)-i-1]
}

func (inc *Queue) Length() int {
	return len(inc.arr)
}

func (inc *Queue) Update(v float64) {
	inc.arr = append(inc.arr, v)
	if len(inc.arr) > inc.size {
		inc.arr = inc.arr[len(inc.arr)-inc.size:]
	}
}

type HeikinAshi struct {
	Close  *Queue
	Open   *Queue
	High   *Queue
	Low    *Queue
	Volume *Queue
}

func NewHeikinAshi(size int) *HeikinAshi {
	return &HeikinAshi{
		Close:  NewQueue(size),
		Open:   NewQueue(size),
		High:   NewQueue(size),
		Low:    NewQueue(size),
		Volume: NewQueue(size),
	}
}

func (s *HeikinAshi) Print() string {
	return fmt.Sprintf("Heikin c: %.3f, o: %.3f, h: %.3f, l: %.3f, v: %.3f",
		s.Close.Last(),
		s.Open.Last(),
		s.High.Last(),
		s.Low.Last(),
		s.Volume.Last())
}

func (inc *HeikinAshi) Update(kline types.KLine) {
	open := kline.Open.Float64()
	cloze := kline.Close.Float64()
	high := kline.High.Float64()
	low := kline.Low.Float64()
	newClose := (open + high + low + cloze) / 4.
	newOpen := (inc.Open.Last() + inc.Close.Last()) / 2.
	inc.Close.Update(newClose)
	inc.Open.Update(newOpen)
	inc.High.Update(math.Max(math.Max(high, newOpen), newClose))
	inc.Low.Update(math.Min(math.Min(low, newOpen), newClose))
	inc.Volume.Update(kline.Volume.Float64())
}

func (s *Strategy) SetupIndicators() {
	store, ok := s.Session.MarketDataStore(s.Symbol)
	if !ok {
		log.Errorf("cannot get marketdatastore of %s", s.Symbol)
		return
	}

	if s.UseHeikinAshi {
		s.heikinAshi = NewHeikinAshi(50)
		store.OnKLineWindowUpdate(func(interval types.Interval, window types.KLineWindow) {
			if s.Interval != interval {
				return
			}
			if s.heikinAshi.Close.Length() == 0 {
				for _, kline := range window {
					s.heikinAshi.Update(kline)
				}
			} else {
				s.heikinAshi.Update(window[len(window)-1])
			}
		})
		if s.UseEma {
			ema5 := &indicator.EWMA{IntervalWindow: types.IntervalWindow{s.Interval, 5}}
			ema34 := &indicator.EWMA{IntervalWindow: types.IntervalWindow{s.Interval, 34}}
			store.OnKLineWindowUpdate(func(interval types.Interval, _ types.KLineWindow) {
				if s.Interval != interval {
					return
				}
				if ema5.Length() == 0 {
					closes := types.ToReverseArray(s.heikinAshi.Close)
					for _, cloze := range closes {
						ema5.Update(cloze)
						ema34.Update(cloze)
					}
				} else {
					cloze := s.heikinAshi.Close.Last()
					ema5.Update(cloze)
					ema34.Update(cloze)
				}
			})
			s.ma5 = ema5
			s.ma34 = ema34
		} else if s.UseSma {
			sma5 := &indicator.SMA{IntervalWindow: types.IntervalWindow{s.Interval, 5}}
			sma34 := &indicator.SMA{IntervalWindow: types.IntervalWindow{s.Interval, 34}}
			store.OnKLineWindowUpdate(func(interval types.Interval, _ types.KLineWindow) {
				if s.Interval != interval {
					return
				}
				if sma5.Length() == 0 {
					closes := types.ToReverseArray(s.heikinAshi.Close)
					for _, cloze := range closes {
						sma5.Update(cloze)
						sma34.Update(cloze)
					}
				} else {
					cloze := s.heikinAshi.Close.Last()
					sma5.Update(cloze)
					sma34.Update(cloze)
				}
			})
			s.ma5 = sma5
			s.ma34 = sma34
		} else {
			evwma5 := &VWEMA{
				PV: &indicator.EWMA{IntervalWindow: types.IntervalWindow{s.Interval, 5}},
				V:  &indicator.EWMA{IntervalWindow: types.IntervalWindow{s.Interval, 5}},
			}
			evwma34 := &VWEMA{
				PV: &indicator.EWMA{IntervalWindow: types.IntervalWindow{s.Interval, 34}},
				V:  &indicator.EWMA{IntervalWindow: types.IntervalWindow{s.Interval, 34}},
			}
			store.OnKLineWindowUpdate(func(interval types.Interval, _ types.KLineWindow) {
				if s.Interval != interval {
					return
				}
				if evwma5.PV.Length() == 0 {
					for i := s.heikinAshi.Close.Length() - 1; i >= 0; i-- {
						price := s.heikinAshi.Close.Index(i)
						vol := s.heikinAshi.Volume.Index(i)
						evwma5.UpdateVal(price, vol)
						evwma34.UpdateVal(price, vol)
					}
				} else {
					price := s.heikinAshi.Close.Last()
					vol := s.heikinAshi.Volume.Last()
					evwma5.UpdateVal(price, vol)
					evwma34.UpdateVal(price, vol)
				}
			})
			s.ma5 = evwma5
			s.ma34 = evwma34
		}
	} else {
		indicatorSet, ok := s.Session.StandardIndicatorSet(s.Symbol)
		if !ok {
			log.Errorf("cannot get indicator set of %s", s.Symbol)
			return
		}
		if s.UseEma {
			s.ma5 = indicatorSet.EWMA(types.IntervalWindow{s.Interval, 5})
			s.ma34 = indicatorSet.EWMA(types.IntervalWindow{s.Interval, 34})
		} else if s.UseSma {
			s.ma5 = indicatorSet.SMA(types.IntervalWindow{s.Interval, 5})
			s.ma34 = indicatorSet.SMA(types.IntervalWindow{s.Interval, 34})
		} else {
			evwma5 := &VWEMA{
				PV: &indicator.EWMA{IntervalWindow: types.IntervalWindow{s.Interval, 5}},
				V:  &indicator.EWMA{IntervalWindow: types.IntervalWindow{s.Interval, 5}},
			}
			evwma34 := &VWEMA{
				PV: &indicator.EWMA{IntervalWindow: types.IntervalWindow{s.Interval, 34}},
				V:  &indicator.EWMA{IntervalWindow: types.IntervalWindow{s.Interval, 34}},
			}
			store.OnKLineWindowUpdate(func(interval types.Interval, window types.KLineWindow) {
				if s.Interval != interval {
					return
				}
				if evwma5.PV.Length() == 0 {
					for _, kline := range window {
						evwma5.Update(kline)
						evwma34.Update(kline)
					}
				} else {
					evwma5.Update(window[len(window)-1])
					evwma34.Update(window[len(window)-1])
				}
			})
			s.ma5 = evwma5
			s.ma34 = evwma34
		}
	}

	s.ewo = types.Mul(types.Minus(types.Div(s.ma5, s.ma34), 1.0), 100.)
	if s.UseEma {
		sig := &indicator.EWMA{IntervalWindow: types.IntervalWindow{s.Interval, s.SignalWindow}}
		store.OnKLineWindowUpdate(func(interval types.Interval, _ types.KLineWindow) {
			if interval != s.Interval {
				return
			}

			if sig.Length() == 0 {
				// lazy init
				ewoVals := types.ToReverseArray(s.ewo)
				for _, ewoValue := range ewoVals {
					sig.Update(ewoValue)
				}
			} else {
				sig.Update(s.ewo.Last())
			}
		})
		s.ewoSignal = sig
	} else if s.UseSma {
		sig := &indicator.SMA{IntervalWindow: types.IntervalWindow{s.Interval, s.SignalWindow}}
		store.OnKLineWindowUpdate(func(interval types.Interval, _ types.KLineWindow) {
			if interval != s.Interval {
				return
			}

			if sig.Length() == 0 {
				// lazy init
				ewoVals := types.ToReverseArray(s.ewo)
				for _, ewoValue := range ewoVals {
					sig.Update(ewoValue)
				}
			} else {
				sig.Update(s.ewo.Last())
			}
		})
		s.ewoSignal = sig
	} else {
		sig := &VWEMA{
			PV: &indicator.EWMA{IntervalWindow: types.IntervalWindow{s.Interval, s.SignalWindow}},
			V:  &indicator.EWMA{IntervalWindow: types.IntervalWindow{s.Interval, s.SignalWindow}},
		}
		store.OnKLineWindowUpdate(func(interval types.Interval, window types.KLineWindow) {
			if interval != s.Interval {
				return
			}
			var vol float64
			if sig.Length() == 0 {
				// lazy init
				ewoVals := types.ToReverseArray(s.ewo)
				for i, ewoValue := range ewoVals {
					if s.UseHeikinAshi {
						vol = s.heikinAshi.Volume.Index(len(ewoVals) - 1 - i)
					} else {
						vol = window[len(ewoVals)-1-i].Volume.Float64()
					}
					sig.PV.Update(ewoValue * vol)
					sig.V.Update(vol)
				}
			} else {
				if s.UseHeikinAshi {
					vol = s.heikinAshi.Volume.Last()
				} else {
					vol = window[len(window)-1].Volume.Float64()
				}
				sig.PV.Update(s.ewo.Last() * vol)
				sig.V.Update(vol)
			}
		})
		s.ewoSignal = sig
	}
}

func (s *Strategy) validateOrder(order *types.SubmitOrder) bool {
	if order.Type == types.OrderTypeMarket && order.TimeInForce != "" {
		return false
	}
	if order.Side == types.SideTypeSell {
		baseBalance, ok := s.Session.GetAccount().Balance(s.Market.BaseCurrency)
		if !ok {
			return false
		}
		if order.Quantity.Compare(baseBalance.Available) > 0 {
			return false
		}
		price := order.Price
		if price.IsZero() {
			price, ok = s.Session.LastPrice(s.Symbol)
			if !ok {
				return false
			}
		}
		orderAmount := order.Quantity.Mul(price)
		if order.Quantity.Sign() <= 0 ||
			order.Quantity.Compare(s.Market.MinQuantity) < 0 ||
			orderAmount.Compare(s.Market.MinNotional) < 0 {
			return false
		}
		return true
	} else if order.Side == types.SideTypeBuy {
		quoteBalance, ok := s.Session.GetAccount().Balance(s.Market.QuoteCurrency)
		if !ok {
			return false
		}
		price := order.Price
		if price.IsZero() {
			price, ok = s.Session.LastPrice(s.Symbol)
			if !ok {
				return false
			}
		}
		totalQuantity := quoteBalance.Available.Div(price)
		if order.Quantity.Compare(totalQuantity) > 0 {
			return false
		}
		orderAmount := order.Quantity.Mul(price)
		if order.Quantity.Sign() <= 0 ||
			orderAmount.Compare(s.Market.MinNotional) < 0 ||
			order.Quantity.Compare(s.Market.MinQuantity) < 0 {
			return false
		}
		return true
	}
	return false

}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	buyPrice := fixedpoint.Zero
	sellPrice := fixedpoint.Zero
	s.peakPrice = fixedpoint.Zero
	s.bottomPrice = fixedpoint.Zero

	orderbook, ok := session.OrderStore(s.Symbol)
	if !ok {
		log.Errorf("cannot get orderbook of %s", s.Symbol)
		return nil
	}
	position, ok := session.Position(s.Symbol)
	if !ok {
		log.Errorf("cannot get position of %s", s.Symbol)
		return nil
	}
	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, position, orderbook)
	s.tradeCollector.OnTrade(func(trade types.Trade, profit, netprofit fixedpoint.Value) {
		if !profit.IsZero() {
			log.Warnf("generate profit: %v, netprofit: %v, trade: %v", profit, netprofit, trade)
		}
		if trade.Side == types.SideTypeBuy {
			buyPrice = trade.Price
			s.peakPrice = buyPrice.Mul(fixedpoint.One.Add(s.Callback))
		} else if trade.Side == types.SideTypeSell {
			sellPrice = trade.Price
			s.bottomPrice = sellPrice.Mul(fixedpoint.One.Sub(s.Callback))
		}
	})

	s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
		log.Infof("position changed: %s", position)
	})
	s.tradeCollector.BindStream(session.UserDataStream)

	s.SmartStops.RunStopControllers(ctx, session, s.tradeCollector)

	s.SetupIndicators()

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != s.Symbol {
			return
		}

		lastPrice, ok := session.LastPrice(s.Symbol)
		if !ok {
			log.Errorf("cannot get last price")
			return
		}
		balances := session.GetAccount().Balances()
		baseBalance := balances[s.Market.BaseCurrency].Available
		quoteBalance := balances[s.Market.QuoteCurrency].Available
		/*if buyPrice.IsZero() {
			if !baseBalance.IsZero() {
				buyPrice = lastPrice
			}
		}
		if sellPrice.IsZero() {
			if !quoteBalance.IsZero() {
				sellPrice = lastPrice
			}
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
		if kline.Interval == types.Interval1m {
			for _, order := range toCancel {
				if order.Side == types.SideTypeBuy && order.Price.Compare(kline.Low) < 0 {
					newPrice := kline.Low
					order.Quantity = order.Quantity.Mul(order.Price).Div(newPrice)
					order.Price = newPrice
					toRepost = append(toRepost, order.SubmitOrder)
				} else if order.Side == types.SideTypeSell && order.Price.Compare(kline.High) > 0 {
					newPrice := kline.High
					order.Price = newPrice
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
			sellall := false
			buyall := false
			if !s.peakPrice.IsZero() {
				change := s.peakPrice.Sub(lastPrice).Div(s.peakPrice)
				if change.Compare(s.Callback) > 0 {
					if !baseBalance.IsZero() {
						sellall = true
					}
					s.peakPrice = fixedpoint.Zero
				} else {
					s.peakPrice = kline.High
				}
			}
			if !s.bottomPrice.IsZero() {
				change := lastPrice.Sub(s.bottomPrice).Div(s.bottomPrice)
				if change.Compare(s.Callback) > 0 {
					if !quoteBalance.IsZero() {
						buyall = true
					}
					s.bottomPrice = fixedpoint.Zero
				} else {
					s.bottomPrice = kline.Low
				}
			}

			if !buyPrice.IsZero() &&
				buyPrice.Sub(lastPrice).Div(buyPrice).Compare(s.Stoploss) > 0 { // stoploss 2%
				sellall = true
			}
			if !sellPrice.IsZero() &&
				lastPrice.Sub(sellPrice).Div(sellPrice).Compare(s.Stoploss) > 0 { // stoploss 2%
				buyall = true
			}
			if sellall {
				balances := session.GetAccount().Balances()
				baseBalance := balances[s.Market.BaseCurrency].Available.Mul(modifier)
				order := types.SubmitOrder{
					Symbol:   s.Symbol,
					Side:     types.SideTypeSell,
					Type:     types.OrderTypeMarket,
					Market:   s.Market,
					Quantity: baseBalance,
				}
				if s.validateOrder(&order) {
					log.Infof("stoploss short at %v, avg %v, timestamp: %s", lastPrice, buyPrice, kline.StartTime)
					createdOrders, err := orderExecutor.SubmitOrders(ctx, order)
					if err != nil {
						log.WithError(err).Errorf("cannot place order")
						return
					}
					log.Infof("stoploss sell order %v", createdOrders)
					s.tradeCollector.Process()
				}
			}

			if buyall {
				quoteBalance, ok := session.GetAccount().Balance(s.Market.QuoteCurrency)
				if !ok {
					return
				}
				quantityAmount := quoteBalance.Available.Mul(modifier)
				totalQuantity := quantityAmount.Div(lastPrice)
				order := types.SubmitOrder{
					Symbol:   kline.Symbol,
					Side:     types.SideTypeBuy,
					Type:     types.OrderTypeMarket,
					Quantity: totalQuantity,
					Market:   s.Market,
				}
				if s.validateOrder(&order) {
					log.Infof("stoploss long at %v, avg %v, timestamp: %s", lastPrice, sellPrice, kline.StartTime)

					createdOrders, err := orderExecutor.SubmitOrders(ctx, order)
					if err != nil {
						log.WithError(err).Errorf("cannot place order")
						return
					}
					log.Infof("stoploss bought order %v", createdOrders)
					s.tradeCollector.Process()
				}
			}
		}

		if kline.Interval != s.Interval {
			return
		}

		// To get the threshold for ewo
		mean := types.Mean(types.Abs(s.ewo), 5)

		longSignal := types.CrossOver(s.ewo, s.ewoSignal)
		shortSignal := types.CrossUnder(s.ewo, s.ewoSignal)
		// get trend flags
		var bull, breakThrough, breakDown bool
		if s.UseHeikinAshi {
			// heikinashi itself contains the concept of trend, so no need breakthrough/down
			bull = s.heikinAshi.Close.Last() > s.heikinAshi.Open.Last()
			breakThrough = true
			breakDown = true
		} else {
			bull = types.Predict(s.ma34, 5, 2) > s.ma34.Last()
			breakThrough = kline.Low.Float64() > s.ma5.Last()
			breakDown = kline.High.Float64() < s.ma5.Last()
		}
		// kline breakthrough ma5, ma50 trend up, and ewo > threshold
		IsBull := bull && breakThrough && s.ewo.Last() >= mean
		// kline downthrough ma5, ma50 trend down, and ewo < threshold
		IsBear := !bull && breakDown && s.ewo.Last() <= -mean

		var orders []types.SubmitOrder
		var price fixedpoint.Value

		if longSignal.Index(1) && !shortSignal.Last() && IsBull {
			if s.UseHeikinAshi {
				price = fixedpoint.NewFromFloat(s.heikinAshi.Close.Last())
			} else {
				price = kline.Low
			}
			quoteBalance, ok := session.GetAccount().Balance(s.Market.QuoteCurrency)
			if !ok {
				return
			}
			quantityAmount := quoteBalance.Available.Mul(modifier)
			totalQuantity := quantityAmount.Div(price)
			order := types.SubmitOrder{
				Symbol:      kline.Symbol,
				Side:        types.SideTypeBuy,
				Type:        types.OrderTypeLimit,
				Price:       price,
				Quantity:    totalQuantity,
				Market:      s.Market,
				TimeInForce: types.TimeInForceGTC,
			}
			if s.validateOrder(&order) {
				// strong long
				log.Warnf("long at %v, timestamp: %s", price, kline.StartTime)

				orders = append(orders, order)
			}
		} else if shortSignal.Index(1) && !longSignal.Last() && IsBear {
			if s.UseHeikinAshi {
				price = fixedpoint.NewFromFloat(s.heikinAshi.Close.Last())
			} else {
				price = kline.High
			}
			balances := session.GetAccount().Balances()
			baseBalance := balances[s.Market.BaseCurrency].Available.Mul(modifier)
			order := types.SubmitOrder{
				Symbol:      s.Symbol,
				Side:        types.SideTypeSell,
				Type:        types.OrderTypeLimit,
				Market:      s.Market,
				Quantity:    baseBalance,
				Price:       price,
				TimeInForce: types.TimeInForceGTC,
			}
			if s.validateOrder(&order) {
				log.Warnf("short at %v, timestamp: %s", price, kline.StartTime)
				orders = append(orders, order)
			}
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
