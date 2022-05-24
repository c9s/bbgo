package ewoDgtrd

import (
	"context"
	"errors"
	"fmt"
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
	Position    *types.Position    `json:"position,omitempty" persistence:"position"`
	ProfitStats *types.ProfitStats `json:"profitStats,omitempty" persistence:"profit_stats"`

	Market           types.Market
	Session          *bbgo.ExchangeSession
	UseHeikinAshi    bool             `json:"useHeikinAshi"` // use heikinashi kline
	Stoploss         fixedpoint.Value `json:"stoploss"`
	Symbol           string           `json:"symbol"`
	Interval         types.Interval   `json:"interval"`
	UseEma           bool             `json:"useEma"`             // use exponential ma or not
	UseSma           bool             `json:"useSma"`             // if UseEma == false, use simple ma or not
	SignalWindow     int              `json:"sigWin"`             // signal window
	DisableShortStop bool             `json:"disableShortStop"`   // disable SL on short
	DisableLongStop  bool             `json:"disableLongStop"`    // disable SL on long
	FilterHigh       float64          `json:"ccistochFilterHigh"` // high filter for CCI Stochastic indicator
	FilterLow        float64          `json:"ccistochFilterLow"`  // low filter for CCI Stochastic indicator

	KLineStartTime types.Time
	KLineEndTime   types.Time

	*bbgo.Environment
	*bbgo.Notifiability
	*bbgo.Persistence
	*bbgo.Graceful
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
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	log.Infof("subscribe %s", s.Symbol)
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})

	session.Subscribe(types.BookTickerChannel, s.Symbol, types.SubscribeOptions{})
}

type UpdatableSeries interface {
	types.Series
	Update(value float64)
}

// Refer: https://tw.tradingview.com/script/XZyG5SOx-CCI-Stochastic-and-a-quick-lesson-on-Scalping-Trading-Systems/
type CCISTOCH struct {
	cci        *indicator.CCI
	stoch      *indicator.STOCH
	ma         *indicator.SMA
	filterHigh float64
	filterLow  float64
}

func NewCCISTOCH(i types.Interval, filterHigh, filterLow float64) *CCISTOCH {
	cci := &indicator.CCI{IntervalWindow: types.IntervalWindow{Interval: i, Window: 28}}
	stoch := &indicator.STOCH{IntervalWindow: types.IntervalWindow{Interval: i, Window: 28}}
	ma := &indicator.SMA{IntervalWindow: types.IntervalWindow{Interval: i, Window: 3}}
	return &CCISTOCH{
		cci:        cci,
		stoch:      stoch,
		ma:         ma,
		filterHigh: filterHigh,
		filterLow:  filterLow,
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
		if v > inc.filterHigh {
			return false
		}
		if v >= inc.filterLow && v <= inc.filterHigh {
			hasGrey = true
			continue
		}
		if v < inc.filterLow {
			return hasGrey
		}
	}
	return false
}

func (inc *CCISTOCH) SellSignal() bool {
	hasGrey := false
	for i := 0; i < len(inc.ma.Values); i++ {
		v := inc.ma.Index(i)
		if v < inc.filterLow {
			return false
		}
		if v >= inc.filterLow && v <= inc.filterHigh {
			hasGrey = true
			continue
		}
		if v > inc.filterHigh {
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

func (s *Strategy) SetupIndicators() {
	store, ok := s.Session.MarketDataStore(s.Symbol)
	if !ok {
		log.Errorf("cannot get marketdatastore of %s", s.Symbol)
		return
	}

	window5 := types.IntervalWindow{Interval: s.Interval, Window: 5}
	window34 := types.IntervalWindow{Interval: s.Interval, Window: 34}
	s.atr = &indicator.ATR{IntervalWindow: window34}
	s.ccis = NewCCISTOCH(s.Interval, s.FilterHigh, s.FilterLow)

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
			ema5 := &indicator.EWMA{IntervalWindow: window5}
			ema34 := &indicator.EWMA{IntervalWindow: window34}
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
			sma5 := &indicator.SMA{IntervalWindow: window5}
			sma34 := &indicator.SMA{IntervalWindow: window34}
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
				PV: &indicator.EWMA{IntervalWindow: window5},
				V:  &indicator.EWMA{IntervalWindow: window5},
			}
			evwma34 := &VWEMA{
				PV: &indicator.EWMA{IntervalWindow: window34},
				V:  &indicator.EWMA{IntervalWindow: window34},
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
			s.ma5 = indicatorSet.EWMA(window5)
			s.ma34 = indicatorSet.EWMA(window34)
		} else if s.UseSma {
			s.ma5 = indicatorSet.SMA(window5)
			s.ma34 = indicatorSet.SMA(window34)
		} else {
			evwma5 := &VWEMA{
				PV: &indicator.EWMA{IntervalWindow: window5},
				V:  &indicator.EWMA{IntervalWindow: window5},
			}
			evwma34 := &VWEMA{
				PV: &indicator.EWMA{IntervalWindow: window34},
				V:  &indicator.EWMA{IntervalWindow: window34},
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
	windowSignal := types.IntervalWindow{Interval: s.Interval, Window: s.SignalWindow}
	if s.UseEma {
		sig := &indicator.EWMA{IntervalWindow: windowSignal}
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
		sig := &indicator.SMA{IntervalWindow: windowSignal}
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
			PV: &indicator.EWMA{IntervalWindow: windowSignal},
			V:  &indicator.EWMA{IntervalWindow: windowSignal},
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

func (s *Strategy) validateOrder(order *types.SubmitOrder) error {
	if order.Type == types.OrderTypeMarket && order.TimeInForce != "" {
		return errors.New("wrong field: market vs TimeInForce")
	}
	if order.Side == types.SideTypeSell {
		baseBalance, ok := s.Session.GetAccount().Balance(s.Market.BaseCurrency)
		if !ok {
			log.Error("cannot get account")
			return errors.New("cannot get account")
		}
		if order.Quantity.Compare(baseBalance.Available) > 0 {
			order.Quantity = baseBalance.Available
		}
		price := order.Price
		if price.IsZero() {
			price, ok = s.Session.LastPrice(s.Symbol)
			if !ok {
				log.Error("no price")
				return errors.New("no price")
			}
		}
		orderAmount := order.Quantity.Mul(price)
		if order.Quantity.Sign() <= 0 ||
			order.Quantity.Compare(s.Market.MinQuantity) < 0 ||
			orderAmount.Compare(s.Market.MinNotional) < 0 {
			log.Debug("amount fail")
			return errors.New("amount fail")
		}
		return nil
	} else if order.Side == types.SideTypeBuy {
		quoteBalance, ok := s.Session.GetAccount().Balance(s.Market.QuoteCurrency)
		if !ok {
			log.Error("cannot get account")
			return errors.New("cannot get account")
		}
		price := order.Price
		if price.IsZero() {
			price, ok = s.Session.LastPrice(s.Symbol)
			if !ok {
				log.Error("no price")
				return errors.New("no price")
			}
		}
		totalQuantity := quoteBalance.Available.Div(price)
		if order.Quantity.Compare(totalQuantity) > 0 {
			log.Error("qty > avail")
			return errors.New("qty > avail")
		}
		orderAmount := order.Quantity.Mul(price)
		if order.Quantity.Sign() <= 0 ||
			orderAmount.Compare(s.Market.MinNotional) < 0 ||
			order.Quantity.Compare(s.Market.MinQuantity) < 0 {
			log.Debug("amount fail")
			return errors.New("amount fail")
		}
		return nil
	}
	log.Error("side error")
	return errors.New("side error")

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
	if err := s.validateOrder(&order); err != nil {
		log.Infof("validation failed %v: %v", order, err)
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
	if err := s.validateOrder(&order); err != nil {
		log.Infof("validation failed %v: %v", order, err)
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
	if err := s.validateOrder(order); err != nil {
		log.Errorf("cannot place close order %v: %v", order, err)
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

func (s *Strategy) CancelAll(ctx context.Context, side types.SideType) {
	var toCancel []types.Order
	for _, order := range s.orderStore.Orders() {
		if order.Status == types.OrderStatusNew || order.Status == types.OrderStatusPartiallyFilled && order.Side == side {
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
// - SL by atr (lastprice < buyprice - atr * 2) || (lastprice > sellprice + atr * 2)
// - TP by cci stoch invert or ewo invert
// - TP by (lastprice < peak price - atr * 2) || (lastprice > bottom price + atr * 2)
// - SL by s.Stoploss (Abs(price_diff / price) > s.Stoploss)
// - entry condition on ewo(Elliott wave oscillator) Crosses ewoSignal(ma on ewo, signalWindow)
//   * buy signal on crossover
//   * sell signal on crossunder
// - and filtered by the following rules:
//   * buy: buy signal ON, kline Close > Open, Close > ma(Window=5), CCI Stochastic Buy signal
//   * sell: sell signal ON, kline Close < Open, Close < ma(Window=5), CCI Stochastic Sell signal
// Cancel non-fully filled orders every s.Interval
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
				s.buyPrice = s.Position.AverageCost
				s.sellPrice = fixedpoint.Zero
				s.peakPrice = s.Position.AverageCost
			} else if sign == 0 {
				log.Infof("base become zero")
				s.buyPrice = fixedpoint.Zero
				s.sellPrice = fixedpoint.Zero
			} else {
				log.Infof("base become negative, %v", trade)
				s.buyPrice = fixedpoint.Zero
				s.sellPrice = s.Position.AverageCost
				s.bottomPrice = s.Position.AverageCost
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

	s.SetupIndicators()

	sellOrderTPSL := func(price fixedpoint.Value) {
		if s.Position.GetBase().IsZero() {
			return
		}
		if s.sellPrice.IsZero() {
			return
		}
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
		if s.bottomPrice.IsZero() || s.bottomPrice.Compare(price) > 0 {
			s.bottomPrice = price
		}
		takeProfit := false
		bottomBack := s.bottomPrice
		spBack := s.sellPrice
		if !quoteBalance.IsZero() {
			longSignal := types.CrossOver(s.ewo, s.ewoSignal)
			// TP
			if lastPrice.Compare(s.sellPrice) < 0 && (s.ccis.BuySignal() || longSignal.Last()) {
				buyall = true
				s.bottomPrice = fixedpoint.Zero
				takeProfit = true
			}
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
			if !s.DisableShortStop && ((!atrx2.IsZero() && s.sellPrice.Add(atrx2).Compare(lastPrice) <= 0) ||
				lastPrice.Sub(s.sellPrice).Div(s.sellPrice).Compare(s.Stoploss) > 0) {
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
		if s.Position.GetBase().IsZero() {
			return
		}
		if s.buyPrice.IsZero() {
			return
		}
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
		if s.peakPrice.IsZero() || s.peakPrice.Compare(price) < 0 {
			s.peakPrice = price
		}
		takeProfit := false
		peakBack := s.peakPrice
		bpBack := s.buyPrice
		if !baseBalance.IsZero() {
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
			if !s.DisableLongStop && (s.buyPrice.Sub(lastPrice).Div(s.buyPrice).Compare(s.Stoploss) > 0 ||
				(!atrx2.IsZero() && s.buyPrice.Sub(atrx2).Compare(lastPrice) >= 0)) {
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
			if s.midPrice.IsZero() {
				lastPrice, ok = session.LastPrice(s.Symbol)
				if !ok {
					log.Errorf("cannot get last price")
					return
				}
			} else {
				lastPrice = s.midPrice
			}
			s.lock.RUnlock()
		}
		balances := session.GetAccount().Balances()
		baseBalance := balances[s.Market.BaseCurrency].Available
		quoteBalance := balances[s.Market.QuoteCurrency].Available
		atr := fixedpoint.NewFromFloat(s.atr.Last())
		if !s.Environment.IsBackTesting() {
			log.Infof("Get last price: %v, ewo %f, ewoSig %f, ccis: %f, atr %v, kline: %v, balance[base]: %v balance[quote]: %v",
				lastPrice, s.ewo.Last(), s.ewoSignal.Last(), s.ccis.ma.Last(), atr, kline, baseBalance, quoteBalance)
		}

		if kline.Interval != s.Interval {
			return
		}

		// To get the threshold for ewo
		// mean := types.Mean(types.Abs(s.ewo), 10)
		// std := types.Stdev(s.ewo, 10)

		longSignal := types.CrossOver(s.ewo, s.ewoSignal)
		shortSignal := types.CrossUnder(s.ewo, s.ewoSignal)
		// get trend flags
		var bull, breakThrough, breakDown bool
		if s.UseHeikinAshi {
			bull = s.heikinAshi.Close.Last() > s.heikinAshi.Open.Last()
			breakThrough = s.heikinAshi.Close.Last() > s.ma5.Last() && s.heikinAshi.Close.Last() > s.ma34.Last()
			breakDown = s.heikinAshi.Close.Last() < s.ma5.Last() && s.heikinAshi.Close.Last() < s.ma34.Last()

		} else {
			bull = kline.Close.Compare(kline.Open) > 0
			breakThrough = kline.Close.Float64() > s.ma5.Last() && kline.Close.Float64() > s.ma34.Last()
			breakDown = kline.Close.Float64() < s.ma5.Last() && kline.Close.Float64() < s.ma34.Last()
		}
		// kline breakthrough ma5, ma34 trend up, and cci Stochastic bull
		IsBull := bull && breakThrough && s.ccis.BuySignal()
		// kline downthrough ma5, ma34 trend down, and cci Stochastic bear
		IsBear := !bull && breakDown && s.ccis.SellSignal()

		if !s.Environment.IsBackTesting() {
			log.Infof("IsBull: %v, bull: %v, longSignal[1]: %v, shortSignal: %v, lastPrice: %v",
				IsBull, bull, longSignal.Index(1), shortSignal.Last(), lastPrice)
			log.Infof("IsBear: %v, bear: %v, shortSignal[1]: %v, longSignal: %v, lastPrice: %v",
				IsBear, !bull, shortSignal.Index(1), longSignal.Last(), lastPrice)
		}

		if longSignal.Index(1) && !shortSignal.Last() && IsBull {
			s.CancelAll(ctx, types.SideTypeBuy)
			if quoteBalance.Div(lastPrice).Compare(s.Market.MinQuantity) >= 0 && quoteBalance.Compare(s.Market.MinNotional) >= 0 {
				s.PlaceBuyOrder(ctx, lastPrice)
			}
		}
		if shortSignal.Index(1) && !longSignal.Last() && IsBear {
			s.CancelAll(ctx, types.SideTypeSell)
			if baseBalance.Mul(lastPrice).Compare(s.Market.MinNotional) >= 0 && baseBalance.Compare(s.Market.MinQuantity) >= 0 {
				s.PlaceSellOrder(ctx, lastPrice)
			}
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
