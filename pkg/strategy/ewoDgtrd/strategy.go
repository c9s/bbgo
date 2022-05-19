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

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Position    *types.Position    `json:"position,omitempty", persistence:"position"`
	ProfitStats *types.ProfitStats `json:"profitStats,omitempty", persistence:"profit_stats"`

	Market           types.Market
	Session          *bbgo.ExchangeSession
	UseHeikinAshi    bool             `json:"useHeikinAshi"` // use heikinashi kline
	Stoploss         fixedpoint.Value `json:"stoploss"`
	Symbol           string           `json:"symbol"`
	Interval         types.Interval   `json:"interval"`
	UseEma           bool             `json:"useEma"`           // use exponential ma or not
	UseSma           bool             `json:"useSma"`           // if UseEma == false, use simple ma or not
	SignalWindow     int              `json:"sigWin"`           // signal window
	DisableShortStop bool             `json:"disableShortStop"` // disable TP/SL on short

	KLineStartTime types.Time
	KLineEndTime   types.Time

	*bbgo.Environment
	*bbgo.Notifiability
	*bbgo.Persistence
	*bbgo.Graceful
	bbgo.SmartStops
	bbgo.StrategyController

	activeMakerOrders *bbgo.LocalActiveOrderBook
	orderStore        *bbgo.OrderStore
	tradeCollector    *bbgo.TradeCollector

	atr         *indicator.ATR
	ccis        *CCISTOCH
	ma5         types.Series
	ma34        types.Series
	ewo         types.Series
	ewoSignal   types.Series
	heikinAshi  *HeikinAshi
	peakPrice   fixedpoint.Value
	bottomPrice fixedpoint.Value
	midPrice    fixedpoint.Value
	lock        sync.RWMutex

	buyPrice  fixedpoint.Value
	sellPrice fixedpoint.Value
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Initialize() error {
	return s.SmartStops.InitializeStopControllers(s.Symbol)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	log.Infof("subscribe %s", s.Symbol)
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})

	session.Subscribe(types.BookTickerChannel, s.Symbol, types.SubscribeOptions{})

	s.SmartStops.Subscribe(session)
}

type UpdatableSeries interface {
	types.Series
	Update(value float64)
}

// Refer: https://tw.tradingview.com/script/XZyG5SOx-CCI-Stochastic-and-a-quick-lesson-on-Scalping-Trading-Systems/
type CCISTOCH struct {
	cci   *indicator.CCI
	stoch *indicator.STOCH
	ma    *indicator.SMA
}

func NewCCISTOCH(i types.Interval) *CCISTOCH {
	cci := &indicator.CCI{IntervalWindow: types.IntervalWindow{i, 28}}
	stoch := &indicator.STOCH{IntervalWindow: types.IntervalWindow{i, 28}}
	ma := &indicator.SMA{IntervalWindow: types.IntervalWindow{i, 3}}
	return &CCISTOCH{
		cci:   cci,
		stoch: stoch,
		ma:    ma,
	}
}

func (inc *CCISTOCH) Update(cloze float64) {
	inc.cci.Update(cloze)
	inc.stoch.Update(inc.cci.Last(), inc.cci.Last(), inc.cci.Last())
	inc.ma.Update(inc.stoch.LastD())
}

func (inc *CCISTOCH) BuySignal() bool {
	hasGrey := false
	for i := 0; i < len(inc.ma.Values); i++ {
		v := inc.ma.Index(i)
		if v > 80 {
			return false
		}
		if v >= 20 && v <= 80 {
			hasGrey = true
			continue
		}
		if v < 20 {
			return hasGrey
		}
	}
	return false
}

func (inc *CCISTOCH) SellSignal() bool {
	hasGrey := false
	for i := 0; i < len(inc.ma.Values); i++ {
		v := inc.ma.Index(i)
		if v < 20 {
			return false
		}
		if v >= 20 && v <= 80 {
			hasGrey = true
			continue
		}
		if v > 80 {
			return hasGrey
		}
	}
	return false
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

	s.atr = &indicator.ATR{IntervalWindow: types.IntervalWindow{s.Interval, 34}}
	s.ccis = NewCCISTOCH(s.Interval)

	if s.UseHeikinAshi {
		s.heikinAshi = NewHeikinAshi(50)
		store.OnKLineWindowUpdate(func(interval types.Interval, window types.KLineWindow) {
			if interval == s.atr.Interval {
				if s.atr.RMA == nil {
					for _, kline := range window {
						s.atr.Update(
							kline.High.Float64(),
							kline.Low.Float64(),
							kline.Close.Float64(),
						)
					}
				} else {
					kline := window[len(window)-1]
					s.atr.Update(
						kline.High.Float64(),
						kline.Low.Float64(),
						kline.Close.Float64(),
					)
				}
			}
			if s.Interval != interval {
				return
			}
			if s.heikinAshi.Close.Length() == 0 {
				for _, kline := range window {
					s.heikinAshi.Update(kline)
					s.ccis.Update(s.heikinAshi.Close.Last())
				}
			} else {
				s.heikinAshi.Update(window[len(window)-1])
				s.ccis.Update(s.heikinAshi.Close.Last())
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

		s.atr.Bind(store)
		store.OnKLineWindowUpdate(func(interval types.Interval, window types.KLineWindow) {
			if s.Interval != interval {
				return
			}
			if s.ccis.cci.Input.Length() == 0 {
				for _, kline := range window {
					s.ccis.Update(kline.Close.Float64())
				}
			} else {
				s.ccis.Update(window[len(window)-1].Close.Float64())
			}
		})
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
			log.Error("cannot get account")
			return false
		}
		if order.Quantity.Compare(baseBalance.Available) > 0 {
			order.Quantity = baseBalance.Available
		}
		price := order.Price
		if price.IsZero() {
			price, ok = s.Session.LastPrice(s.Symbol)
			if !ok {
				log.Error("no price")
				return false
			}
		}
		orderAmount := order.Quantity.Mul(price)
		if order.Quantity.Sign() <= 0 ||
			order.Quantity.Compare(s.Market.MinQuantity) < 0 ||
			orderAmount.Compare(s.Market.MinNotional) < 0 {
			log.Debug("amount fail")
			return false
		}
		return true
	} else if order.Side == types.SideTypeBuy {
		quoteBalance, ok := s.Session.GetAccount().Balance(s.Market.QuoteCurrency)
		if !ok {
			log.Error("cannot get account")
			return false
		}
		price := order.Price
		if price.IsZero() {
			price, ok = s.Session.LastPrice(s.Symbol)
			if !ok {
				log.Error("no price")
				return false
			}
		}
		totalQuantity := quoteBalance.Available.Div(price)
		if order.Quantity.Compare(totalQuantity) > 0 {
			log.Error("qty > avail")
			return false
		}
		orderAmount := order.Quantity.Mul(price)
		if order.Quantity.Sign() <= 0 ||
			orderAmount.Compare(s.Market.MinNotional) < 0 ||
			order.Quantity.Compare(s.Market.MinQuantity) < 0 {
			log.Debug("amount fail")
			return false
		}
		return true
	}
	log.Error("side error")
	return false

}

func (s *Strategy) PlaceBuyOrder(ctx context.Context, price fixedpoint.Value) {
	if s.Position.GetBase().Add(s.Market.MinQuantity).Sign() < 0 && !s.ClosePosition(ctx) {
		log.Errorf("sell position %v remained not closed, skip placing order", s.Position.GetBase())
		return
	}
	quoteBalance, ok := s.Session.GetAccount().Balance(s.Market.QuoteCurrency)
	if !ok {
		log.Infof("buy order at price %v failed", price)
		return
	}
	quantityAmount := quoteBalance.Available
	totalQuantity := quantityAmount.Div(price)
	order := types.SubmitOrder{
		Symbol:      s.Symbol,
		Side:        types.SideTypeBuy,
		Type:        types.OrderTypeLimit,
		Price:       price,
		Quantity:    totalQuantity,
		Market:      s.Market,
		TimeInForce: types.TimeInForceGTC,
	}
	if !s.validateOrder(&order) {
		log.Debugf("validation failed %v", order)
		return
	}
	// strong long
	log.Warnf("long at %v, timestamp: %s", price, s.KLineStartTime)
	createdOrders, err := s.Session.Exchange.SubmitOrders(ctx, order)
	if err != nil {
		log.WithError(err).Errorf("cannot place order")
		return
	}
	log.Infof("post order %v", createdOrders)
	s.orderStore.Add(createdOrders...)
	s.activeMakerOrders.Add(createdOrders...)
	s.tradeCollector.Process()
}

func (s *Strategy) PlaceSellOrder(ctx context.Context, price fixedpoint.Value) {
	if s.Position.GetBase().Compare(s.Market.MinQuantity) > 0 && !s.ClosePosition(ctx) {
		log.Errorf("buy position %v remained not closed, skip placing order", s.Position.GetBase())
		return
	}
	balances := s.Session.GetAccount().Balances()
	baseBalance := balances[s.Market.BaseCurrency].Available
	order := types.SubmitOrder{
		Symbol:      s.Symbol,
		Side:        types.SideTypeSell,
		Type:        types.OrderTypeLimit,
		Market:      s.Market,
		Quantity:    baseBalance,
		Price:       price,
		TimeInForce: types.TimeInForceGTC,
	}
	if !s.validateOrder(&order) {
		log.Debugf("validation failed %v", order)
		return
	}

	log.Warnf("short at %v, timestamp: %s", price, s.KLineStartTime)
	createdOrders, err := s.Session.Exchange.SubmitOrders(ctx, order)
	if err != nil {
		log.WithError(err).Errorf("cannot place order")
		return
	}
	log.Infof("post order %v", createdOrders)
	s.orderStore.Add(createdOrders...)
	s.activeMakerOrders.Add(createdOrders...)
	s.tradeCollector.Process()
}

func (s *Strategy) ClosePosition(ctx context.Context) bool {
	order := s.Position.NewClosePositionOrder(fixedpoint.One)
	if order == nil {
		// no base
		s.sellPrice = fixedpoint.Zero
		s.buyPrice = fixedpoint.Zero
		return true
	}
	order.TimeInForce = ""
	if !s.validateOrder(order) {
		log.Errorf("cannot place close order %v", order)
		return false
	}

	createdOrders, err := s.Session.Exchange.SubmitOrders(ctx, *order)
	if err != nil {
		log.WithError(err).Errorf("cannot place close order")
		return false
	}
	log.Infof("close order %v", createdOrders)
	s.orderStore.Add(createdOrders...)
	s.activeMakerOrders.Add(createdOrders...)
	s.tradeCollector.Process()
	return true
}

func (s *Strategy) CancelAll(ctx context.Context) {
	var toCancel []types.Order
	for _, order := range s.orderStore.Orders() {
		if order.Status == types.OrderStatusNew || order.Status == types.OrderStatusPartiallyFilled {
			toCancel = append(toCancel, order)
		}
	}
	if len(toCancel) > 0 {
		if err := s.Session.Exchange.CancelOrders(ctx, toCancel...); err != nil {
			log.WithError(err).Errorf("cancel order error")
		}

		s.tradeCollector.Process()
	}
}

// Trading Rules:
// - buy / sell the whole asset
// - SL/TP by atr (buyprice - 2 * atr, sellprice + 2 * atr)
// - SL by s.Stoploss (Abs(price_diff / price) > s.Stoploss)
// - entry condition on ewo(Elliott wave oscillator) Crosses ewoSignal(ma on ewo, signalWindow)
//   * buy signal on crossover
//   * sell signal on crossunder
// - and filtered by the following rules:
//   * buy: prev buy signal ON and current sell signal OFF, kline Close > Open, Close > ma(Window=5), CCI Stochastic Buy signal
//   * sell: prev buy signal OFF and current sell signal ON, kline Close < Open, Close < ma(Window=5), CCI Stochastic Sell signal
// Cancel non-fully filed orders every bar
//
// ps: kline might refer to heikinashi or normal ohlc
func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.buyPrice = fixedpoint.Zero
	s.sellPrice = fixedpoint.Zero
	s.peakPrice = fixedpoint.Zero
	s.bottomPrice = fixedpoint.Zero

	s.activeMakerOrders = bbgo.NewLocalActiveOrderBook(s.Symbol)
	s.activeMakerOrders.BindStream(session.UserDataStream)

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.UserDataStream)

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}
	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}
	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.Position, s.orderStore)
	s.tradeCollector.OnTrade(func(trade types.Trade, profit, netprofit fixedpoint.Value) {
		if s.Symbol != trade.Symbol {
			return
		}
		s.Notifiability.Notify(trade)
		s.ProfitStats.AddTrade(trade)

		if !profit.IsZero() {
			log.Warnf("generate profit: %v, netprofit: %v, trade: %v", profit, netprofit, trade)
			p := s.Position.NewProfit(trade, profit, netprofit)
			p.Strategy = ID
			p.StrategyInstanceID = s.InstanceID()
			s.Notify(&p)

			s.ProfitStats.AddProfit(p)
			s.Notify(&s.ProfitStats)
			s.Environment.RecordPosition(s.Position, trade, &p)
		} else {
			s.Environment.RecordPosition(s.Position, trade, nil)
		}
		if s.Position.GetBase().Abs().Compare(s.Market.MinQuantity) > 0 {
			sign := s.Position.GetBase().Sign()
			if sign > 0 {
				log.Infof("base become positive, %v", trade)
				s.buyPrice = trade.Price
				s.peakPrice = trade.Price
			} else if sign == 0 {
				log.Infof("base become zero")
				s.buyPrice = fixedpoint.Zero
				s.sellPrice = fixedpoint.Zero
			} else {
				log.Infof("base become negative, %v", trade)
				s.sellPrice = trade.Price
				s.bottomPrice = trade.Price
			}
		} else {
			log.Infof("base become zero")
			s.buyPrice = fixedpoint.Zero
			s.sellPrice = fixedpoint.Zero
		}
	})

	s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
		log.Infof("position changed: %s", position)
		s.Notify(s.Position)
	})
	s.tradeCollector.BindStream(session.UserDataStream)

	s.SmartStops.RunStopControllers(ctx, session, s.tradeCollector)

	s.SetupIndicators()

	sellOrderTPSL := func(price fixedpoint.Value) {
		balances := session.GetAccount().Balances()
		quoteBalance := balances[s.Market.QuoteCurrency].Available
		atrx2 := fixedpoint.NewFromFloat(s.atr.Last() * 2)
		lastPrice := price
		var ok bool
		if s.Environment.IsBackTesting() {
			lastPrice, ok = session.LastPrice(s.Symbol)
			if !ok {
				log.Errorf("cannot get last price")
				return
			}
		}
		buyall := false
		if !s.sellPrice.IsZero() {
			if s.bottomPrice.IsZero() || s.bottomPrice.Compare(price) > 0 {
				s.bottomPrice = price
			}
		}
		takeProfit := false
		bottomBack := s.bottomPrice
		spBack := s.sellPrice
		if !quoteBalance.IsZero() && !s.sellPrice.IsZero() && !s.DisableShortStop {
			// longSignal := types.CrossOver(s.ewo, s.ewoSignal)
			// TP
			/*if lastPrice.Compare(s.sellPrice) < 0 && (s.ccis.BuySignal() || longSignal.Last())  {
				buyall = true
				s.bottomPrice = fixedpoint.Zero
				takeProfit = true
			}*/
			if !atrx2.IsZero() && s.bottomPrice.Add(atrx2).Compare(lastPrice) >= 0 &&
				lastPrice.Compare(s.sellPrice) < 0 {
				buyall = true
				s.bottomPrice = fixedpoint.Zero
				takeProfit = true
			}

			// SL
			/*if (!atrx2.IsZero() && s.bottomPrice.Add(atrx2).Compare(lastPrice) <= 0) ||
				lastPrice.Sub(s.bottomPrice).Div(lastPrice).Compare(s.Stoploss) > 0 {
				if lastPrice.Compare(s.sellPrice) < 0 {
					takeProfit = true
				}
				buyall = true
				s.bottomPrice = fixedpoint.Zero
			}*/
			if (!atrx2.IsZero() && s.sellPrice.Add(atrx2).Compare(lastPrice) <= 0) ||
				lastPrice.Sub(s.sellPrice).Div(s.sellPrice).Compare(s.Stoploss) > 0 {
				buyall = true
				s.bottomPrice = fixedpoint.Zero
			}
		}
		if buyall {
			log.Warnf("buyall TPSL %v %v", s.Position.GetBase(), quoteBalance)
			if s.ClosePosition(ctx) {
				if takeProfit {
					log.Errorf("takeprofit buy at %v, avg %v, l: %v, atrx2: %v", lastPrice, spBack, bottomBack, atrx2)
				} else {
					log.Errorf("stoploss buy at %v, avg %v, l: %v, atrx2: %v", lastPrice, spBack, bottomBack, atrx2)
				}
			}
		}
	}
	buyOrderTPSL := func(price fixedpoint.Value) {
		balances := session.GetAccount().Balances()
		baseBalance := balances[s.Market.BaseCurrency].Available
		atrx2 := fixedpoint.NewFromFloat(s.atr.Last() * 2)
		lastPrice := price
		var ok bool
		if s.Environment.IsBackTesting() {
			lastPrice, ok = session.LastPrice(s.Symbol)
			if !ok {
				log.Errorf("cannot get last price")
				return
			}
		}
		sellall := false
		if !s.buyPrice.IsZero() {
			if s.peakPrice.IsZero() || s.peakPrice.Compare(price) < 0 {
				s.peakPrice = price
			}
		}
		takeProfit := false
		peakBack := s.peakPrice
		bpBack := s.buyPrice
		if !baseBalance.IsZero() && !s.buyPrice.IsZero() {
			shortSignal := types.CrossUnder(s.ewo, s.ewoSignal)
			// TP
			if !atrx2.IsZero() && s.peakPrice.Sub(atrx2).Compare(lastPrice) >= 0 &&
				lastPrice.Compare(s.buyPrice) > 0 {
				sellall = true
				s.peakPrice = fixedpoint.Zero
				takeProfit = true
			}
			if lastPrice.Compare(s.buyPrice) > 0 && (s.ccis.SellSignal() || shortSignal.Last()) {
				sellall = true
				s.peakPrice = fixedpoint.Zero
				takeProfit = true
			}

			// SL
			/*if s.peakPrice.Sub(lastPrice).Div(s.peakPrice).Compare(s.Stoploss) > 0 ||
				(!atrx2.IsZero() && s.peakPrice.Sub(atrx2).Compare(lastPrice) >= 0) {
				if lastPrice.Compare(s.buyPrice) > 0 {
					takeProfit = true
				}
				sellall = true
				s.peakPrice = fixedpoint.Zero
			}*/
			if s.buyPrice.Sub(lastPrice).Div(s.buyPrice).Compare(s.Stoploss) > 0 ||
				(!atrx2.IsZero() && s.buyPrice.Sub(atrx2).Compare(lastPrice) >= 0) {
				sellall = true
				s.peakPrice = fixedpoint.Zero
			}
		}

		if sellall {
			log.Warnf("sellall TPSL %v", s.Position.GetBase())
			if s.ClosePosition(ctx) {
				if takeProfit {
					log.Errorf("takeprofit sell at %v, avg %v, h: %v, atrx2: %v", lastPrice, bpBack, peakBack, atrx2)
				} else {
					log.Errorf("stoploss sell at %v, avg %v, h: %v, atrx2: %v", lastPrice, bpBack, peakBack, atrx2)
				}
			}
		}
	}

	// set last price by realtime book ticker update
	// to trigger TP/SL
	session.MarketDataStream.OnBookTickerUpdate(func(ticker types.BookTicker) {
		if s.Environment.IsBackTesting() {
			return
		}
		bestBid := ticker.Buy
		bestAsk := ticker.Sell
		var midPrice fixedpoint.Value

		// TODO: for go1.18 we can use TryLock, use build flag to support this
		if tryLock(&s.lock) {
			if !bestAsk.IsZero() && !bestBid.IsZero() {
				s.midPrice = bestAsk.Add(bestBid).Div(types.Two)
			} else if !bestAsk.IsZero() {
				s.midPrice = bestAsk
			} else {
				s.midPrice = bestBid
			}
			midPrice = s.midPrice
			s.lock.Unlock()
		}

		if !midPrice.IsZero() {
			buyOrderTPSL(midPrice)
			sellOrderTPSL(midPrice)
			// log.Debugf("best bid %v, best ask %v, mid %v", bestBid, bestAsk, midPrice)
		}
	})

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != s.Symbol {
			return
		}
		s.KLineStartTime = kline.StartTime
		s.KLineEndTime = kline.EndTime

		// well, only track prices on 1m
		if kline.Interval == types.Interval1m {

			if s.Environment.IsBackTesting() {
				buyOrderTPSL(kline.High)
				sellOrderTPSL(kline.Low)

			}
		}

		var lastPrice fixedpoint.Value
		var ok bool
		if s.Environment.IsBackTesting() {
			lastPrice, ok = session.LastPrice(s.Symbol)
			if !ok {
				log.Errorf("cannot get last price")
				return
			}
		} else {
			s.lock.RLock()
			lastPrice = s.midPrice
			s.lock.RUnlock()
		}
		if !s.Environment.IsBackTesting() {
			balances := session.GetAccount().Balances()
			baseBalance := balances[s.Market.BaseCurrency].Available
			quoteBalance := balances[s.Market.QuoteCurrency].Available
			atrx2 := fixedpoint.NewFromFloat(s.atr.Last() * 2)
			log.Infof("Get last price: %v, ewo %f, ewoSig %f, ccis: %f, atrx2 %v, kline: %v, balance[base]: %v balance[quote]: %v",
				lastPrice, s.ewo.Last(), s.ewoSignal.Last(), s.ccis.ma.Last(), atrx2, kline, baseBalance, quoteBalance)
		}

		if kline.Interval != s.Interval {
			return
		}

		s.CancelAll(ctx)

		// To get the threshold for ewo
		// mean := types.Mean(s.ewo, 10)
		// std := types.Stdev(s.ewo, 10)

		longSignal := types.CrossOver(s.ewo, s.ewoSignal)
		shortSignal := types.CrossUnder(s.ewo, s.ewoSignal)
		// get trend flags
		var bull, breakThrough, breakDown bool
		if s.UseHeikinAshi {
			bull = s.heikinAshi.Close.Last() > s.heikinAshi.Open.Last()
			breakThrough = s.heikinAshi.Close.Last() > s.ma5.Last()
			breakDown = s.heikinAshi.Close.Last() < s.ma5.Last()
		} else {
			bull = kline.Close.Compare(kline.Open) > 0
			breakThrough = kline.Close.Float64() > s.ma5.Last()
			breakDown = kline.Close.Float64() < s.ma5.Last()
		}
		// kline breakthrough ma5, ma50 trend up, and ewo > threshold
		IsBull := bull && breakThrough && s.ccis.BuySignal() // && s.ewo.Last() > mean + 2 * std
		// kline downthrough ma5, ma50 trend down, and ewo < threshold
		IsBear := !bull && breakDown && s.ccis.SellSignal() // .ewo.Last() < mean - 2 * std

		if !s.Environment.IsBackTesting() {
			log.Infof("IsBull: %v, bull: %v, longSignal[1]: %v, shortSignal: %v",
				IsBull, bull, longSignal.Index(1), shortSignal.Last())
			log.Infof("IsBear: %v, bear: %v, shortSignal[1]: %v, longSignal: %v",
				IsBear, !bull, shortSignal.Index(1), longSignal.Last())
		}

		price := lastPrice
		if longSignal.Index(1) && !shortSignal.Last() && IsBull {
			s.PlaceBuyOrder(ctx, price)
		} else if shortSignal.Index(1) && !longSignal.Last() && IsBear {
			s.PlaceSellOrder(ctx, price)
		}
	})
	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		log.Infof("canceling active orders...")

		var toCancel []types.Order
		for _, order := range s.orderStore.Orders() {
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
