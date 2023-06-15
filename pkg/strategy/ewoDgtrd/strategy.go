package ewoDgtrd

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

const ID = "ewo_dgtrd"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	// Embedded components
	// ===================
	*bbgo.Environment
	bbgo.StrategyController

	// Auto-Injection fields
	// ====================

	// Market of the symbol
	Market types.Market

	// Session is the trading session of this strategy
	Session *bbgo.ExchangeSession

	orderExecutor *bbgo.GeneralOrderExecutor

	// Persistence fields
	// ====================
	// Position
	Position *types.Position `json:"position,omitempty" persistence:"position"`

	// ProfitStats
	ProfitStats *types.ProfitStats `json:"profitStats,omitempty" persistence:"profit_stats"`

	// Settings fields
	// =========================
	UseHeikinAshi       bool             `json:"useHeikinAshi"` // use heikinashi kline
	StopLoss            fixedpoint.Value `json:"stoploss"`
	Symbol              string           `json:"symbol"`
	Interval            types.Interval   `json:"interval"`
	UseEma              bool             `json:"useEma"`              // use exponential ma or not
	UseSma              bool             `json:"useSma"`              // if UseEma == false, use simple ma or not
	SignalWindow        int              `json:"sigWin"`              // signal window
	DisableShortStop    bool             `json:"disableShortStop"`    // disable SL on short
	DisableLongStop     bool             `json:"disableLongStop"`     // disable SL on long
	FilterHigh          float64          `json:"cciStochFilterHigh"`  // high filter for CCI Stochastic indicator
	FilterLow           float64          `json:"cciStochFilterLow"`   // low filter for CCI Stochastic indicator
	EwoChangeFilterHigh float64          `json:"ewoChangeFilterHigh"` // high filter for ewo histogram
	EwoChangeFilterLow  float64          `json:"ewoChangeFilterLow"`  // low filter for ewo histogram

	Record bool `json:"record"` // print record messages on position exit point

	KLineStartTime types.Time
	KLineEndTime   types.Time

	entryPrice   fixedpoint.Value
	waitForTrade bool

	atr           *indicator.ATR
	emv           *indicator.EMV
	ccis          *CCISTOCH
	ma5           types.SeriesExtend
	ma34          types.SeriesExtend
	ewo           types.SeriesExtend
	ewoSignal     types.SeriesExtend
	ewoHistogram  types.SeriesExtend
	ewoChangeRate float64
	heikinAshi    *HeikinAshi
	peakPrice     fixedpoint.Value
	bottomPrice   fixedpoint.Value
	midPrice      fixedpoint.Value
	lock          sync.RWMutex

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
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})

	if !bbgo.IsBackTesting {
		session.Subscribe(types.BookTickerChannel, s.Symbol, types.SubscribeOptions{})
	}
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
	inc.stoch.Update(inc.cci.Last(0), inc.cci.Last(0), inc.cci.Last(0))
	inc.ma.Update(inc.stoch.LastD())
}

func (inc *CCISTOCH) BuySignal() bool {
	hasGrey := false
	for i := 0; i < inc.ma.Values.Length(); i++ {
		v := inc.ma.Index(i)
		if v > inc.filterHigh {
			return false
		} else if v >= inc.filterLow && v <= inc.filterHigh {
			hasGrey = true
			continue
		} else if v < inc.filterLow {
			return hasGrey
		}
	}
	return false
}

func (inc *CCISTOCH) SellSignal() bool {
	hasGrey := false
	for i := 0; i < inc.ma.Values.Length(); i++ {
		v := inc.ma.Index(i)
		if v < inc.filterLow {
			return false
		} else if v >= inc.filterLow && v <= inc.filterHigh {
			hasGrey = true
			continue
		} else if v > inc.filterHigh {
			return hasGrey
		}
	}
	return false
}

type VWEMA struct {
	PV types.UpdatableSeries
	V  types.UpdatableSeries
}

func (inc *VWEMA) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *VWEMA) Last(i int) float64 {
	if i >= inc.PV.Length() {
		return 0
	}
	vi := inc.V.Last(i)
	if vi == 0 {
		return 0
	}
	return inc.PV.Last(i) / vi
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

// Setup the Indicators going to be used
func (s *Strategy) SetupIndicators(store *bbgo.MarketDataStore) {
	window5 := types.IntervalWindow{Interval: s.Interval, Window: 5}
	window34 := types.IntervalWindow{Interval: s.Interval, Window: 34}
	s.atr = &indicator.ATR{IntervalWindow: window34}
	s.emv = &indicator.EMV{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: 14}}
	s.ccis = NewCCISTOCH(s.Interval, s.FilterHigh, s.FilterLow)

	getSource := func(window types.KLineWindow) types.Series {
		if s.UseHeikinAshi {
			return s.heikinAshi.Close
		}
		return window.Close()
	}
	getVol := func(window types.KLineWindow) types.Series {
		if s.UseHeikinAshi {
			return s.heikinAshi.Volume
		}
		return window.Volume()
	}
	s.heikinAshi = NewHeikinAshi(500)
	store.OnKLineWindowUpdate(func(interval types.Interval, window types.KLineWindow) {
		if interval == s.atr.Interval {
			if s.atr.RMA == nil {
				for _, kline := range window {
					high := kline.High.Float64()
					low := kline.Low.Float64()
					cloze := kline.Close.Float64()
					vol := kline.Volume.Float64()
					s.atr.Update(high, low, cloze)
					s.emv.Update(high, low, vol)
				}
			} else {
				kline := window[len(window)-1]
				high := kline.High.Float64()
				low := kline.Low.Float64()
				cloze := kline.Close.Float64()
				vol := kline.Volume.Float64()
				s.atr.Update(high, low, cloze)
				s.emv.Update(high, low, vol)
			}
		}
		if s.Interval != interval {
			return
		}
		if s.heikinAshi.Close.Length() == 0 {
			for _, kline := range window {
				s.heikinAshi.Update(kline)
				s.ccis.Update(getSource(window).Last(0))
			}
		} else {
			s.heikinAshi.Update(window[len(window)-1])
			s.ccis.Update(getSource(window).Last(0))
		}
	})
	if s.UseEma {
		ema5 := &indicator.EWMA{IntervalWindow: window5}
		ema34 := &indicator.EWMA{IntervalWindow: window34}
		store.OnKLineWindowUpdate(func(interval types.Interval, window types.KLineWindow) {
			if s.Interval != interval {
				return
			}
			if ema5.Length() == 0 {
				closes := types.Reverse(getSource(window))
				for _, cloze := range closes {
					ema5.Update(cloze)
					ema34.Update(cloze)
				}
			} else {
				cloze := getSource(window).Last(0)
				ema5.Update(cloze)
				ema34.Update(cloze)
			}

		})

		s.ma5 = ema5
		s.ma34 = ema34
	} else if s.UseSma {
		sma5 := &indicator.SMA{IntervalWindow: window5}
		sma34 := &indicator.SMA{IntervalWindow: window34}
		store.OnKLineWindowUpdate(func(interval types.Interval, window types.KLineWindow) {
			if s.Interval != interval {
				return
			}
			if sma5.Length() == 0 {
				closes := types.Reverse(getSource(window))
				for _, cloze := range closes {
					sma5.Update(cloze)
					sma34.Update(cloze)
				}
			} else {
				cloze := getSource(window).Last(0)
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
		store.OnKLineWindowUpdate(func(interval types.Interval, window types.KLineWindow) {
			if s.Interval != interval {
				return
			}
			clozes := getSource(window)
			vols := getVol(window)
			if evwma5.PV.Length() == 0 {
				for i := clozes.Length() - 1; i >= 0; i-- {
					price := clozes.Last(i)
					vol := vols.Last(i)
					evwma5.UpdateVal(price, vol)
					evwma34.UpdateVal(price, vol)
				}
			} else {
				price := clozes.Last(0)
				vol := vols.Last(0)
				evwma5.UpdateVal(price, vol)
				evwma34.UpdateVal(price, vol)
			}
		})
		s.ma5 = types.NewSeries(evwma5)
		s.ma34 = types.NewSeries(evwma34)
	}

	s.ewo = s.ma5.Div(s.ma34).Minus(1.0).Mul(100.)
	s.ewoHistogram = s.ma5.Minus(s.ma34)
	windowSignal := types.IntervalWindow{Interval: s.Interval, Window: s.SignalWindow}
	if s.UseEma {
		sig := &indicator.EWMA{IntervalWindow: windowSignal}
		store.OnKLineWindowUpdate(func(interval types.Interval, _ types.KLineWindow) {
			if interval != s.Interval {
				return
			}

			if sig.Length() == 0 {
				// lazy init
				ewoVals := types.Reverse(s.ewo)
				for _, ewoValue := range ewoVals {
					sig.Update(ewoValue)
				}
			} else {
				sig.Update(s.ewo.Last(0))
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
				ewoVals := s.ewo.Reverse()
				for _, ewoValue := range ewoVals {
					sig.Update(ewoValue)
				}
			} else {
				sig.Update(s.ewo.Last(0))
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
			if sig.Length() == 0 {
				// lazy init
				ewoVals := s.ewo.Reverse()
				for i, ewoValue := range ewoVals {
					vol := window.Volume().Last(i)
					sig.PV.Update(ewoValue * vol)
					sig.V.Update(vol)
				}
			} else {
				vol := window.Volume().Last(0)
				sig.PV.Update(s.ewo.Last(0) * vol)
				sig.V.Update(vol)
			}
		})
		s.ewoSignal = types.NewSeries(sig)
	}
}

// Utility to evaluate if the order is valid or not to send to the exchange
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
			log.Errorf("qty %v > avail %v", order.Quantity, baseBalance.Available)
			return errors.New("qty > avail")
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
			return fmt.Errorf("amount fail: quantity: %v, amount: %v", order.Quantity, orderAmount)
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
			log.Errorf("qty %v > avail %v", order.Quantity, totalQuantity)
			return errors.New("qty > avail")
		}
		orderAmount := order.Quantity.Mul(price)
		if order.Quantity.Sign() <= 0 ||
			orderAmount.Compare(s.Market.MinNotional) < 0 ||
			order.Quantity.Compare(s.Market.MinQuantity) < 0 {
			log.Debug("amount fail")
			return fmt.Errorf("amount fail: quantity: %v, amount: %v", order.Quantity, orderAmount)
		}
		return nil
	}
	log.Error("side error")
	return errors.New("side error")

}

func (s *Strategy) PlaceBuyOrder(ctx context.Context, price fixedpoint.Value) (*types.Order, *types.Order) {
	var closeOrder *types.Order
	var ok bool
	waitForTrade := false
	base := s.Position.GetBase()
	if base.Abs().Compare(s.Market.MinQuantity) >= 0 && base.Mul(s.GetLastPrice()).Abs().Compare(s.Market.MinNotional) >= 0 && base.Sign() < 0 {
		if closeOrder, ok = s.ClosePosition(ctx); !ok {
			log.Errorf("sell position %v remained not closed, skip placing order", base)
			return closeOrder, nil
		}
	}
	if s.Position.GetBase().Sign() < 0 {
		// we are not able to make close trade at this moment,
		// will close the rest of the position by normal limit order
		// s.entryPrice is set in the last trade
		waitForTrade = true
	}
	quoteBalance, ok := s.Session.GetAccount().Balance(s.Market.QuoteCurrency)
	if !ok {
		log.Infof("buy order at price %v failed", price)
		return closeOrder, nil
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
		return closeOrder, nil
	}
	log.Warnf("long at %v, position %v, closeOrder %v, timestamp: %s", price, s.Position.GetBase(), closeOrder, s.KLineStartTime)

	createdOrders, err := s.orderExecutor.SubmitOrders(ctx, order)
	if err != nil {
		log.WithError(err).Errorf("cannot place order")
		return closeOrder, nil
	}

	log.Infof("post order c: %v, entryPrice: %v o: %v", waitForTrade, s.entryPrice, createdOrders)
	s.waitForTrade = waitForTrade
	return closeOrder, &createdOrders[0]
}

func (s *Strategy) PlaceSellOrder(ctx context.Context, price fixedpoint.Value) (*types.Order, *types.Order) {
	var closeOrder *types.Order
	var ok bool
	waitForTrade := false
	base := s.Position.GetBase()
	if base.Abs().Compare(s.Market.MinQuantity) >= 0 && base.Abs().Mul(s.GetLastPrice()).Compare(s.Market.MinNotional) >= 0 && base.Sign() > 0 {
		if closeOrder, ok = s.ClosePosition(ctx); !ok {
			log.Errorf("buy position %v remained not closed, skip placing order", base)
			return closeOrder, nil
		}
	}
	if s.Position.GetBase().Sign() > 0 {
		// we are not able to make close trade at this moment,
		// will close the rest of the position by normal limit order
		// s.entryPrice is set in the last trade
		waitForTrade = true
	}
	baseBalance, ok := s.Session.GetAccount().Balance(s.Market.BaseCurrency)
	if !ok {
		return closeOrder, nil
	}
	order := types.SubmitOrder{
		Symbol:      s.Symbol,
		Side:        types.SideTypeSell,
		Type:        types.OrderTypeLimit,
		Market:      s.Market,
		Quantity:    baseBalance.Available,
		Price:       price,
		TimeInForce: types.TimeInForceGTC,
	}
	if err := s.validateOrder(&order); err != nil {
		log.Infof("validation failed %v: %v", order, err)
		return closeOrder, nil
	}

	log.Warnf("short at %v, position %v closeOrder %v, timestamp: %s", price, s.Position.GetBase(), closeOrder, s.KLineStartTime)
	createdOrders, err := s.orderExecutor.SubmitOrders(ctx, order)
	if err != nil {
		log.WithError(err).Errorf("cannot place order")
		return closeOrder, nil
	}

	log.Infof("post order, c: %v, entryPrice: %v o: %v", waitForTrade, s.entryPrice, createdOrders)
	s.waitForTrade = waitForTrade
	return closeOrder, &createdOrders[0]
}

// ClosePosition(context.Context) -> (closeOrder *types.Order, ok bool)
// this will decorate the generated order from NewMarketCloseOrder
// add do necessary checks
// if available quantity is zero, will return (nil, true)
// if any of the checks failed, will return (nil, false)
// otherwise, return the created close order and true
func (s *Strategy) ClosePosition(ctx context.Context) (*types.Order, bool) {
	order := s.Position.NewMarketCloseOrder(fixedpoint.One)
	// no position exists
	if order == nil {
		// no base
		s.sellPrice = fixedpoint.Zero
		s.buyPrice = fixedpoint.Zero
		return nil, true
	}
	order.TimeInForce = ""
	// If there's any order not yet been traded in the orderbook,
	// we need this additional check to make sure we have enough balance to post a close order
	balances := s.Session.GetAccount().Balances()
	baseBalance := balances[s.Market.BaseCurrency].Available
	if order.Side == types.SideTypeBuy {
		price := s.GetLastPrice()
		quoteAmount := balances[s.Market.QuoteCurrency].Available.Div(price)
		if order.Quantity.Compare(quoteAmount) > 0 {
			order.Quantity = quoteAmount
		}
	} else if order.Side == types.SideTypeSell && order.Quantity.Compare(baseBalance) > 0 {
		order.Quantity = baseBalance
	}

	// if no available balance...
	if order.Quantity.IsZero() {
		return nil, true
	}
	if err := s.validateOrder(order); err != nil {
		log.Errorf("cannot place close order %v: %v", order, err)
		return nil, false
	}

	createdOrders, err := s.orderExecutor.SubmitOrders(ctx, *order)
	if err != nil {
		log.WithError(err).Errorf("cannot place close order")
		return nil, false
	}

	log.Infof("close order %v", createdOrders)
	return &createdOrders[0], true
}

func (s *Strategy) CancelAll(ctx context.Context) {
	if err := s.orderExecutor.GracefulCancel(ctx); err != nil {
		log.WithError(err).Errorf("graceful cancel order error")
	}

	s.waitForTrade = false
}

func (s *Strategy) GetLastPrice() fixedpoint.Value {
	var lastPrice fixedpoint.Value
	var ok bool
	if s.Environment.IsBackTesting() {
		lastPrice, ok = s.Session.LastPrice(s.Symbol)
		if !ok {
			log.Errorf("cannot get last price")
			return lastPrice
		}
	} else {
		s.lock.RLock()
		if s.midPrice.IsZero() {
			lastPrice, ok = s.Session.LastPrice(s.Symbol)
			if !ok {
				log.Errorf("cannot get last price")
				return lastPrice
			}
		} else {
			lastPrice = s.midPrice
		}
		s.lock.RUnlock()
	}
	return lastPrice
}

// Trading Rules:
// - buy / sell the whole asset
// - SL by atr (lastprice < buyprice - atr) || (lastprice > sellprice + atr)
// - TP by detecting if there's a ewo pivotHigh(1,1) -> close long, or pivotLow(1,1) -> close short
// - TP by ma34 +- atr * 2
// - TP by (lastprice < peak price - atr) || (lastprice > bottom price + atr)
// - SL by s.StopLoss (Abs(price_diff / price) > s.StopLoss)
// - entry condition on ewo(Elliott wave oscillator) Crosses ewoSignal(ma on ewo, signalWindow)
//   - buy signal on (crossover on previous K bar and no crossunder on latest K bar)
//   - sell signal on (crossunder on previous K bar and no crossunder on latest K bar)
//
// - and filtered by the following rules:
//   - buy: buy signal ON, kline Close > Open, Close > ma5, Close > ma34, CCI Stochastic Buy signal
//   - sell: sell signal ON, kline Close < Open, Close < ma5, Close < ma34, CCI Stochastic Sell signal
//
// - or entry when ma34 +- atr * 3 gets touched
// - entry price: latestPrice +- atr / 2 (short,long), close at market price
// Cancel non-fully filled orders on new signal (either in same direction or not)
//
// ps: kline might refer to heikinashi or normal ohlc
func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.buyPrice = fixedpoint.Zero
	s.sellPrice = fixedpoint.Zero
	s.peakPrice = fixedpoint.Zero
	s.bottomPrice = fixedpoint.Zero

	counterTPfromPeak := 0
	percentAvgTPfromPeak := 0.0
	counterTPfromCCI := 0
	percentAvgTPfromCCI := 0.0
	counterTPfromLongShort := 0
	percentAvgTPfromLongShort := 0.0
	counterTPfromAtr := 0
	percentAvgTPfromAtr := 0.0
	counterTPfromOrder := 0
	percentAvgTPfromOrder := 0.0
	counterSLfromSL := 0
	percentAvgSLfromSL := 0.0
	counterSLfromOrder := 0
	percentAvgSLfromOrder := 0.0

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	instanceID := s.InstanceID()
	s.orderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.orderExecutor.BindEnvironment(s.Environment)
	s.orderExecutor.BindProfitStats(s.ProfitStats)
	// s.orderExecutor.BindTradeStats(s.TradeStats)
	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})
	s.orderExecutor.Bind()

	s.orderExecutor.TradeCollector().OnTrade(func(trade types.Trade, profit, netprofit fixedpoint.Value) {
		// calculate report for the position that cannot be closed by close order (amount too small)
		if s.waitForTrade {
			price := s.entryPrice
			if price.IsZero() {
				panic("no price found")
			}
			pnlRate := trade.Price.Sub(price).Abs().Div(trade.Price).Float64()
			if s.Record {
				log.Errorf("record avg %v trade %v", price, trade)
			}
			if trade.Side == types.SideTypeBuy {
				if trade.Price.Compare(price) < 0 {
					percentAvgTPfromOrder = percentAvgTPfromOrder*float64(counterTPfromOrder) + pnlRate
					counterTPfromOrder += 1
					percentAvgTPfromOrder /= float64(counterTPfromOrder)
				} else {
					percentAvgSLfromOrder = percentAvgSLfromOrder*float64(counterSLfromOrder) + pnlRate
					counterSLfromOrder += 1
					percentAvgSLfromOrder /= float64(counterSLfromOrder)
				}
			} else if trade.Side == types.SideTypeSell {
				if trade.Price.Compare(price) > 0 {
					percentAvgTPfromOrder = percentAvgTPfromOrder*float64(counterTPfromOrder) + pnlRate
					counterTPfromOrder += 1
					percentAvgTPfromOrder /= float64(counterTPfromOrder)
				} else {
					percentAvgSLfromOrder = percentAvgSLfromOrder*float64(counterSLfromOrder) + pnlRate
					counterSLfromOrder += 1
					percentAvgSLfromOrder /= float64(counterSLfromOrder)
				}
			} else {
				panic(fmt.Sprintf("no sell(%v) or buy price(%v), %v", s.sellPrice, s.buyPrice, trade))
			}
			s.waitForTrade = false
		}

		if s.Position.GetBase().Abs().Compare(s.Market.MinQuantity) >= 0 && s.Position.GetBase().Abs().Mul(trade.Price).Compare(s.Market.MinNotional) >= 0 {
			sign := s.Position.GetBase().Sign()
			if sign > 0 {
				log.Infof("base become positive, %v", trade)
				s.buyPrice = s.Position.AverageCost
				s.sellPrice = fixedpoint.Zero
				s.peakPrice = s.Position.AverageCost
			} else if sign == 0 {
				panic("not going to happen")
			} else {
				log.Infof("base become negative, %v", trade)
				s.buyPrice = fixedpoint.Zero
				s.sellPrice = s.Position.AverageCost
				s.bottomPrice = s.Position.AverageCost
			}
			s.entryPrice = trade.Price
		} else {
			log.Infof("base become zero, rest of base: %v", s.Position.GetBase())
			if s.Position.GetBase().IsZero() {
				s.entryPrice = fixedpoint.Zero
			}
			s.buyPrice = fixedpoint.Zero
			s.sellPrice = fixedpoint.Zero
			s.peakPrice = fixedpoint.Zero
			s.bottomPrice = fixedpoint.Zero
		}
	})

	store, ok := s.Session.MarketDataStore(s.Symbol)
	if !ok {
		return fmt.Errorf("cannot get marketdatastore of %s", s.Symbol)
	}
	s.SetupIndicators(store)

	// local peak of ewo
	shortSig := s.ewo.Last(0) < s.ewo.Last(1) && s.ewo.Last(1) > s.ewo.Last(2)
	longSig := s.ewo.Last(0) > s.ewo.Last(1) && s.ewo.Last(1) < s.ewo.Last(2)

	sellOrderTPSL := func(price fixedpoint.Value) {
		lastPrice := s.GetLastPrice()
		base := s.Position.GetBase().Abs()
		if base.Mul(lastPrice).Compare(s.Market.MinNotional) < 0 || base.Compare(s.Market.MinQuantity) < 0 {
			return
		}
		if s.sellPrice.IsZero() {
			return
		}
		balances := session.GetAccount().Balances()
		quoteBalance := balances[s.Market.QuoteCurrency].Available
		atr := fixedpoint.NewFromFloat(s.atr.Last(0))
		atrx2 := fixedpoint.NewFromFloat(s.atr.Last(0) * 2)
		buyall := false
		if s.bottomPrice.IsZero() || s.bottomPrice.Compare(price) > 0 {
			s.bottomPrice = price
		}
		takeProfit := false
		bottomBack := s.bottomPrice
		spBack := s.sellPrice
		reason := -1
		if quoteBalance.Div(lastPrice).Compare(s.Market.MinQuantity) >= 0 && quoteBalance.Compare(s.Market.MinNotional) >= 0 {
			base := fixedpoint.NewFromFloat(s.ma34.Last(0))
			// TP
			if lastPrice.Compare(s.sellPrice) < 0 && (longSig ||
				(!atrx2.IsZero() && base.Sub(atrx2).Compare(lastPrice) >= 0)) {
				buyall = true
				takeProfit = true

				// calculate report
				if longSig {
					reason = 1
				} else {
					reason = 2
				}

			}
			if !atr.IsZero() && s.bottomPrice.Add(atr).Compare(lastPrice) <= 0 &&
				lastPrice.Compare(s.sellPrice) < 0 {
				buyall = true
				takeProfit = true
				reason = 3
			}

			// SL
			/*if (!atrx2.IsZero() && s.bottomPrice.Add(atrx2).Compare(lastPrice) <= 0) ||
				lastPrice.Sub(s.bottomPrice).Div(lastPrice).Compare(s.StopLoss) > 0 {
				if lastPrice.Compare(s.sellPrice) < 0 {
					takeProfit = true
				}
				buyall = true
				s.bottomPrice = fixedpoint.Zero
			}*/
			if !s.DisableShortStop && ((!atr.IsZero() && s.sellPrice.Sub(atr).Compare(lastPrice) >= 0) ||
				lastPrice.Sub(s.sellPrice).Div(s.sellPrice).Compare(s.StopLoss) > 0) {
				buyall = true
				reason = 4
			}
		}
		if buyall {
			log.Warnf("buyall TPSL %v %v", s.Position.GetBase(), quoteBalance)
			p := s.sellPrice
			if order, ok := s.ClosePosition(ctx); order != nil && ok {
				if takeProfit {
					log.Errorf("takeprofit buy at %v, avg %v, l: %v, atrx2: %v", lastPrice, spBack, bottomBack, atrx2)
				} else {
					log.Errorf("stoploss buy at %v, avg %v, l: %v, atrx2: %v", lastPrice, spBack, bottomBack, atrx2)
				}

				// calculate report
				if s.Record {
					log.Error("record ba")
				}
				var pnlRate float64
				if takeProfit {
					pnlRate = p.Sub(lastPrice).Div(lastPrice).Float64()
				} else {
					pnlRate = lastPrice.Sub(p).Div(lastPrice).Float64()
				}
				switch reason {
				case 0:
					percentAvgTPfromCCI = percentAvgTPfromCCI*float64(counterTPfromCCI) + pnlRate
					counterTPfromCCI += 1
					percentAvgTPfromCCI /= float64(counterTPfromCCI)
				case 1:
					percentAvgTPfromLongShort = percentAvgTPfromLongShort*float64(counterTPfromLongShort) + pnlRate
					counterTPfromLongShort += 1
					percentAvgTPfromLongShort /= float64(counterTPfromLongShort)
				case 2:
					percentAvgTPfromAtr = percentAvgTPfromAtr*float64(counterTPfromAtr) + pnlRate
					counterTPfromAtr += 1
					percentAvgTPfromAtr /= float64(counterTPfromAtr)
				case 3:
					percentAvgTPfromPeak = percentAvgTPfromPeak*float64(counterTPfromPeak) + pnlRate
					counterTPfromPeak += 1
					percentAvgTPfromPeak /= float64(counterTPfromPeak)
				case 4:
					percentAvgSLfromSL = percentAvgSLfromSL*float64(counterSLfromSL) + pnlRate
					counterSLfromSL += 1
					percentAvgSLfromSL /= float64(counterSLfromSL)

				}
			}
		}
	}
	buyOrderTPSL := func(price fixedpoint.Value) {
		lastPrice := s.GetLastPrice()
		base := s.Position.GetBase().Abs()
		if base.Mul(lastPrice).Compare(s.Market.MinNotional) < 0 || base.Compare(s.Market.MinQuantity) < 0 {
			return
		}
		if s.buyPrice.IsZero() {
			return
		}
		balances := session.GetAccount().Balances()
		baseBalance := balances[s.Market.BaseCurrency].Available
		atr := fixedpoint.NewFromFloat(s.atr.Last(0))
		atrx2 := fixedpoint.NewFromFloat(s.atr.Last(0) * 2)
		sellall := false
		if s.peakPrice.IsZero() || s.peakPrice.Compare(price) < 0 {
			s.peakPrice = price
		}
		takeProfit := false
		peakBack := s.peakPrice
		bpBack := s.buyPrice
		reason := -1
		if baseBalance.Compare(s.Market.MinQuantity) >= 0 && baseBalance.Mul(lastPrice).Compare(s.Market.MinNotional) >= 0 {
			// TP
			base := fixedpoint.NewFromFloat(s.ma34.Last(0))
			if lastPrice.Compare(s.buyPrice) > 0 && (shortSig ||
				(!atrx2.IsZero() && base.Add(atrx2).Compare(lastPrice) <= 0)) {
				sellall = true
				takeProfit = true

				// calculate report
				if shortSig {
					reason = 1
				} else {
					reason = 2
				}
			}
			if !atr.IsZero() && s.peakPrice.Sub(atr).Compare(lastPrice) >= 0 &&
				lastPrice.Compare(s.buyPrice) > 0 {
				sellall = true
				takeProfit = true
				reason = 3
			}

			// SL
			/*if s.peakPrice.Sub(lastPrice).Div(s.peakPrice).Compare(s.StopLoss) > 0 ||
				(!atrx2.IsZero() && s.peakPrice.Sub(atrx2).Compare(lastPrice) >= 0) {
				if lastPrice.Compare(s.buyPrice) > 0 {
					takeProfit = true
				}
				sellall = true
				s.peakPrice = fixedpoint.Zero
			}*/
			if !s.DisableLongStop && (s.buyPrice.Sub(lastPrice).Div(s.buyPrice).Compare(s.StopLoss) > 0 ||
				(!atr.IsZero() && s.buyPrice.Sub(atr).Compare(lastPrice) >= 0)) {
				sellall = true
				reason = 4
			}
		}

		if sellall {
			log.Warnf("sellall TPSL %v", s.Position.GetBase())
			p := s.buyPrice
			if order, ok := s.ClosePosition(ctx); order != nil && ok {
				if takeProfit {
					log.Errorf("takeprofit sell at %v, avg %v, h: %v, atrx2: %v", lastPrice, bpBack, peakBack, atrx2)
				} else {
					log.Errorf("stoploss sell at %v, avg %v, h: %v, atrx2: %v", lastPrice, bpBack, peakBack, atrx2)
				}
				// calculate report
				if s.Record {
					log.Error("record sa")
				}
				var pnlRate float64
				if takeProfit {
					pnlRate = lastPrice.Sub(p).Div(p).Float64()
				} else {
					pnlRate = p.Sub(lastPrice).Div(p).Float64()
				}
				switch reason {
				case 0:
					percentAvgTPfromCCI = percentAvgTPfromCCI*float64(counterTPfromCCI) + pnlRate
					counterTPfromCCI += 1
					percentAvgTPfromCCI /= float64(counterTPfromCCI)
				case 1:
					percentAvgTPfromLongShort = percentAvgTPfromLongShort*float64(counterTPfromLongShort) + pnlRate
					counterTPfromLongShort += 1
					percentAvgTPfromLongShort /= float64(counterTPfromLongShort)
				case 2:
					percentAvgTPfromAtr = percentAvgTPfromAtr*float64(counterTPfromAtr) + pnlRate
					counterTPfromAtr += 1
					percentAvgTPfromAtr /= float64(counterTPfromAtr)
				case 3:
					percentAvgTPfromPeak = percentAvgTPfromPeak*float64(counterTPfromPeak) + pnlRate
					counterTPfromPeak += 1
					percentAvgTPfromPeak /= float64(counterTPfromPeak)
				case 4:
					percentAvgSLfromSL = percentAvgSLfromSL*float64(counterSLfromSL) + pnlRate
					counterSLfromSL += 1
					percentAvgSLfromSL /= float64(counterSLfromSL)
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

		if util.TryLock(&s.lock) {
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

	getHigh := func(window types.KLineWindow) types.Series {
		if s.UseHeikinAshi {
			return s.heikinAshi.High
		}
		return window.High()
	}
	getLow := func(window types.KLineWindow) types.Series {
		if s.UseHeikinAshi {
			return s.heikinAshi.Low
		}
		return window.Low()
	}
	getClose := func(window types.KLineWindow) types.Series {
		if s.UseHeikinAshi {
			return s.heikinAshi.Close
		}
		return window.Close()
	}
	getOpen := func(window types.KLineWindow) types.Series {
		if s.UseHeikinAshi {
			return s.heikinAshi.Open
		}
		return window.Open()
	}

	store.OnKLineWindowUpdate(func(interval types.Interval, window types.KLineWindow) {
		kline := window[len(window)-1]
		s.KLineStartTime = kline.StartTime
		s.KLineEndTime = kline.EndTime

		// well, only track prices on 1m
		if interval == types.Interval1m {

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
		baseBalance := balances[s.Market.BaseCurrency].Total()
		quoteBalance := balances[s.Market.QuoteCurrency].Total()
		atr := fixedpoint.NewFromFloat(s.atr.Last(0))
		if !s.Environment.IsBackTesting() {
			log.Infof("Get last price: %v, ewo %f, ewoSig %f, ccis: %f, atr %v, kline: %v, balance[base]: %v balance[quote]: %v",
				lastPrice, s.ewo.Last(0), s.ewoSignal.Last(0), s.ccis.ma.Last(0), atr, kline, baseBalance, quoteBalance)
		}

		if kline.Interval != s.Interval {
			return
		}

		priceHighest := types.Highest(getHigh(window), 233)
		priceLowest := types.Lowest(getLow(window), 233)
		priceChangeRate := (priceHighest - priceLowest) / priceHighest / 14
		ewoHighest := types.Highest(s.ewoHistogram, 233)

		s.ewoChangeRate = math.Abs(s.ewoHistogram.Last(0) / ewoHighest * priceChangeRate)

		longSignal := types.CrossOver(s.ewo, s.ewoSignal)
		shortSignal := types.CrossUnder(s.ewo, s.ewoSignal)

		base := s.ma34.Last(0)
		sellLine := base + s.atr.Last(0)*3
		buyLine := base - s.atr.Last(0)*3
		clozes := getClose(window)
		opens := getOpen(window)

		// get trend flags
		bull := clozes.Last(0) > opens.Last(0)
		breakThrough := clozes.Last(0) > s.ma5.Last(0) && clozes.Last(0) > s.ma34.Last(0)
		breakDown := clozes.Last(0) < s.ma5.Last(0) && clozes.Last(0) < s.ma34.Last(0)

		// kline breakthrough ma5, ma34 trend up, and cci Stochastic bull
		IsBull := bull && breakThrough && s.ccis.BuySignal() && s.ewoChangeRate < s.EwoChangeFilterHigh && s.ewoChangeRate > s.EwoChangeFilterLow
		// kline downthrough ma5, ma34 trend down, and cci Stochastic bear
		IsBear := !bull && breakDown && s.ccis.SellSignal() && s.ewoChangeRate < s.EwoChangeFilterHigh && s.ewoChangeRate > s.EwoChangeFilterLow

		if !s.Environment.IsBackTesting() {
			log.Infof("IsBull: %v, bull: %v, longSignal[1]: %v, shortSignal: %v, lastPrice: %v",
				IsBull, bull, longSignal.Index(1), shortSignal.Last(), lastPrice)
			log.Infof("IsBear: %v, bear: %v, shortSignal[1]: %v, longSignal: %v, lastPrice: %v",
				IsBear, !bull, shortSignal.Index(1), longSignal.Last(), lastPrice)
		}

		if (longSignal.Index(1) && !shortSignal.Last() && IsBull) || lastPrice.Float64() <= buyLine {
			price := lastPrice.Sub(atr.Div(types.Two))
			// if total asset (including locked) could be used to buy
			if quoteBalance.Div(price).Compare(s.Market.MinQuantity) >= 0 && quoteBalance.Compare(s.Market.MinNotional) >= 0 {
				// cancel all orders to release lock
				s.CancelAll(ctx)

				// backup, since the s.sellPrice will be cleared when doing ClosePosition
				sellPrice := s.sellPrice
				log.Errorf("ewoChangeRate %v, emv %v", s.ewoChangeRate, s.emv.Last(0))

				// calculate report
				if closeOrder, _ := s.PlaceBuyOrder(ctx, price); closeOrder != nil {
					if s.Record {
						log.Error("record l")
					}
					if !sellPrice.IsZero() {
						if lastPrice.Compare(sellPrice) > 0 {
							pnlRate := lastPrice.Sub(sellPrice).Div(lastPrice).Float64()
							percentAvgTPfromOrder = percentAvgTPfromOrder*float64(counterTPfromOrder) + pnlRate
							counterTPfromOrder += 1
							percentAvgTPfromOrder /= float64(counterTPfromOrder)
						} else {
							pnlRate := sellPrice.Sub(lastPrice).Div(lastPrice).Float64()
							percentAvgSLfromOrder = percentAvgSLfromOrder*float64(counterSLfromOrder) + pnlRate
							counterSLfromOrder += 1
							percentAvgSLfromOrder /= float64(counterSLfromOrder)
						}
					} else {
						panic("no sell price")
					}
				}
			}
		}
		if (shortSignal.Index(1) && !longSignal.Last() && IsBear) || lastPrice.Float64() >= sellLine {
			price := lastPrice.Add(atr.Div(types.Two))
			// if total asset (including locked) could be used to sell
			if baseBalance.Mul(price).Compare(s.Market.MinNotional) >= 0 && baseBalance.Compare(s.Market.MinQuantity) >= 0 {
				// cancel all orders to release lock
				s.CancelAll(ctx)

				// backup, since the s.buyPrice will be cleared when doing ClosePosition
				buyPrice := s.buyPrice
				log.Errorf("ewoChangeRate: %v, emv %v", s.ewoChangeRate, s.emv.Last(0))

				// calculate report
				if closeOrder, _ := s.PlaceSellOrder(ctx, price); closeOrder != nil {
					if s.Record {
						log.Error("record s")
					}
					if !buyPrice.IsZero() {
						if lastPrice.Compare(buyPrice) > 0 {
							pnlRate := lastPrice.Sub(buyPrice).Div(buyPrice).Float64()
							percentAvgTPfromOrder = percentAvgTPfromOrder*float64(counterTPfromOrder) + pnlRate
							counterTPfromOrder += 1
							percentAvgTPfromOrder /= float64(counterTPfromOrder)
						} else {
							pnlRate := buyPrice.Sub(lastPrice).Div(buyPrice).Float64()
							percentAvgSLfromOrder = percentAvgSLfromOrder*float64(counterSLfromOrder) + pnlRate
							counterSLfromOrder += 1
							percentAvgSLfromOrder /= float64(counterSLfromOrder)
						}
					} else {
						panic("no buy price")
					}
				}
			}
		}
	})

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		log.Infof("canceling active orders...")

		_ = s.orderExecutor.GracefulCancel(ctx)

		hiblue := color.New(color.FgHiBlue).FprintfFunc()
		blue := color.New(color.FgBlue).FprintfFunc()
		hiyellow := color.New(color.FgHiYellow).FprintfFunc()
		hiblue(os.Stderr, "---- Trade Report (Without Fee) ----\n")
		hiblue(os.Stderr, "TP:\n")
		blue(os.Stderr, "\tpeak / bottom with atr: %d, avg pnl rate: %f\n", counterTPfromPeak, percentAvgTPfromPeak)
		blue(os.Stderr, "\tCCI Stochastic: %d, avg pnl rate: %f\n", counterTPfromCCI, percentAvgTPfromCCI)
		blue(os.Stderr, "\tLongSignal/ShortSignal: %d, avg pnl rate: %f\n", counterTPfromLongShort, percentAvgTPfromLongShort)
		blue(os.Stderr, "\tma34 and Atrx2: %d, avg pnl rate: %f\n", counterTPfromAtr, percentAvgTPfromAtr)
		blue(os.Stderr, "\tActive Order: %d, avg pnl rate: %f\n", counterTPfromOrder, percentAvgTPfromOrder)

		totalTP := counterTPfromPeak + counterTPfromCCI + counterTPfromLongShort + counterTPfromAtr + counterTPfromOrder
		avgProfit := (float64(counterTPfromPeak)*percentAvgTPfromPeak +
			float64(counterTPfromCCI)*percentAvgTPfromCCI +
			float64(counterTPfromLongShort)*percentAvgTPfromLongShort +
			float64(counterTPfromAtr)*percentAvgTPfromAtr +
			float64(counterTPfromOrder)*percentAvgTPfromOrder) / float64(totalTP)
		hiblue(os.Stderr, "\tSum: %d, avg pnl rate: %f\n", totalTP, avgProfit)

		hiblue(os.Stderr, "SL:\n")
		blue(os.Stderr, "\tentry SL: %d, avg pnl rate: -%f\n", counterSLfromSL, percentAvgSLfromSL)
		blue(os.Stderr, "\tActive Order: %d, avg pnl rate: -%f\n", counterSLfromOrder, percentAvgSLfromOrder)

		totalSL := counterSLfromSL + counterSLfromOrder
		avgLoss := (float64(counterSLfromSL)*percentAvgSLfromSL + float64(counterSLfromOrder)*percentAvgSLfromOrder) / float64(totalSL)
		hiblue(os.Stderr, "\tSum: %d, avg pnl rate: -%f\n", totalSL, avgLoss)

		hiblue(os.Stderr, "WinRate: %f\n", float64(totalTP)/float64(totalTP+totalSL))

		maString := "vwema"
		if s.UseSma {
			maString = "sma"
		}
		if s.UseEma {
			maString = "ema"
		}

		hiyellow(os.Stderr, "----- EWO Settings -------\n")
		hiyellow(os.Stderr, "General:\n")
		hiyellow(os.Stderr, "\tuseHeikinAshi: %v\n", s.UseHeikinAshi)
		hiyellow(os.Stderr, "\tstoploss: %v\n", s.StopLoss)
		hiyellow(os.Stderr, "\tsymbol: %s\n", s.Symbol)
		hiyellow(os.Stderr, "\tinterval: %s\n", s.Interval)
		hiyellow(os.Stderr, "\tMA type: %s\n", maString)
		hiyellow(os.Stderr, "\tdisableShortStop: %v\n", s.DisableShortStop)
		hiyellow(os.Stderr, "\tdisableLongStop: %v\n", s.DisableLongStop)
		hiyellow(os.Stderr, "\trecord: %v\n", s.Record)
		hiyellow(os.Stderr, "CCI Stochastic:\n")
		hiyellow(os.Stderr, "\tccistochFilterHigh: %f\n", s.FilterHigh)
		hiyellow(os.Stderr, "\tccistochFilterLow: %f\n", s.FilterLow)
		hiyellow(os.Stderr, "Ewo && Ewo Histogram:\n")
		hiyellow(os.Stderr, "\tsigWin: %d\n", s.SignalWindow)
		hiyellow(os.Stderr, "\tewoChngFilterHigh: %f\n", s.EwoChangeFilterHigh)
		hiyellow(os.Stderr, "\tewoChngFilterLow: %f\n", s.EwoChangeFilterLow)
	})
	return nil
}
