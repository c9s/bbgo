package drift

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/dynamic"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/interact"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

const ID = "drift"

var log = logrus.WithField("strategy", ID)
var Four fixedpoint.Value = fixedpoint.NewFromInt(4)
var Three fixedpoint.Value = fixedpoint.NewFromInt(3)
var Two fixedpoint.Value = fixedpoint.NewFromInt(2)
var Delta fixedpoint.Value = fixedpoint.NewFromFloat(0.01)
var Fee = 0.0008 // taker fee % * 2, for upper bound

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

func filterErrors(errs []error) (es []error) {
	for _, e := range errs {
		if _, ok := e.(types.ZeroAssetError); ok {
			continue
		}
		if bbgo.ErrExceededSubmitOrderRetryLimit == e {
			continue
		}
		es = append(es, e)
	}
	return es
}

type Strategy struct {
	Symbol string `json:"symbol"`

	bbgo.OpenPositionOptions
	bbgo.StrategyController
	types.Market
	types.IntervalWindow
	bbgo.SourceSelector

	*bbgo.Environment
	*types.Position    `persistence:"position"`
	*types.ProfitStats `persistence:"profit_stats"`
	*types.TradeStats  `persistence:"trade_stats"`

	MinInterval types.Interval `json:"MinInterval"` // minimum interval referred for doing stoploss/trailing exists and updating highest/lowest

	elapsed                *types.Queue
	priceLines             *types.Queue
	trendLine              types.UpdatableSeriesExtend
	ma                     types.UpdatableSeriesExtend
	stdevHigh              *indicator.StdDev
	stdevLow               *indicator.StdDev
	drift                  *DriftMA
	atr                    *indicator.ATR
	midPrice               fixedpoint.Value // the midPrice is the average of bestBid and bestAsk in public orderbook
	lock                   sync.RWMutex     `ignore:"true"` // lock for midPrice
	positionLock           sync.RWMutex     `ignore:"true"` // lock for highest/lowest and p
	pendingLock            sync.Mutex       `ignore:"true"`
	startTime              time.Time        // trading start time
	maxCounterBuyCanceled  int              // the largest counter of the order on the buy side been cancelled. meaning the latest cancelled buy order.
	maxCounterSellCanceled int              // the largest counter of the order on the sell side been cancelled. meaning the latest cancelled sell order.
	orderPendingCounter    map[uint64]int   // records the timepoint when the orders are created, using the counter at the time.
	frameKLine             *types.KLine     // last kline in Interval
	klineMin               *types.KLine     // last kline in MinInterval

	beta float64 // last beta value from trendline's linear regression (previous slope of the trendline)

	Debug                     bool             `json:"debug" modifiable:"true"`                // to print debug message or not
	UseStopLoss               bool             `json:"useStopLoss" modifiable:"true"`          // whether to use stoploss rate to do stoploss
	UseAtr                    bool             `json:"useAtr" modifiable:"true"`               // use atr as stoploss
	StopLoss                  fixedpoint.Value `json:"stoploss" modifiable:"true"`             // stoploss rate
	PredictOffset             int              `json:"predictOffset"`                          // the lookback length for the prediction using linear regression
	HighLowVarianceMultiplier float64          `json:"hlVarianceMultiplier" modifiable:"true"` // modifier to set the limit order price
	NoTrailingStopLoss        bool             `json:"noTrailingStopLoss" modifiable:"true"`   // turn off the trailing exit and stoploss
	HLRangeWindow             int              `json:"hlRangeWindow"`                          // ma window for kline high/low changes
	SmootherWindow            int              `json:"smootherWindow"`                         // window that controls the smoothness of drift
	FisherTransformWindow     int              `json:"fisherTransformWindow"`                  // fisher transform window to filter drift's negative signals
	ATRWindow                 int              `json:"atrWindow"`                              // window for atr indicator
	PendingMinInterval        int              `json:"pendingMinInterval" modifiable:"true"`   // if order not be traded for pendingMinInterval of time, cancel it.
	NoRebalance               bool             `json:"noRebalance" modifiable:"true"`          // disable rebalance
	TrendWindow               int              `json:"trendWindow"`                            // trendLine is used for rebalancing the position. When trendLine goes up, hold base, otherwise hold quote
	RebalanceFilter           float64          `json:"rebalanceFilter" modifiable:"true"`      // beta filter on the Linear Regression of trendLine
	TrailingCallbackRate      []float64        `json:"trailingCallbackRate" modifiable:"true"`
	TrailingActivationRatio   []float64        `json:"trailingActivationRatio" modifiable:"true"`

	buyPrice     float64 `persistence:"buy_price"`     // price when a long position is opened
	sellPrice    float64 `persistence:"sell_price"`    // price when a short position is opened
	highestPrice float64 `persistence:"highest_price"` // highestPrice when the position is opened
	lowestPrice  float64 `persistence:"lowest_price"`  // lowestPrice when the position is opened

	// This is not related to trade but for statistics graph generation
	// Will deduct fee in percentage from every trade
	GraphPNLDeductFee bool   `json:"graphPNLDeductFee"`
	CanvasPath        string `json:"canvasPath"`       // backtest related. the path to store the indicator graph
	GraphPNLPath      string `json:"graphPNLPath"`     // backtest related. the path to store the pnl % graph per trade graph.
	GraphCumPNLPath   string `json:"graphCumPNLPath"`  // backtest related. the path to store the asset changes in graph
	GraphElapsedPath  string `json:"graphElapsedPath"` // the path to store the elapsed time in ms
	GenerateGraph     bool   `json:"generateGraph"`    // whether to generate graph when shutdown

	ExitMethods bbgo.ExitMethodSet `json:"exits"`
	Session     *bbgo.ExchangeSession

	*bbgo.FastOrderExecutor

	getLastPrice func() fixedpoint.Value
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s:%v", ID, s.Symbol, bbgo.IsBackTesting)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// by default, bbgo only pre-subscribe 1000 klines.
	// this is not enough if we're subscribing 30m intervals using SerialMarketDataStore

	if !bbgo.IsBackTesting {
		session.Subscribe(types.BookTickerChannel, s.Symbol, types.SubscribeOptions{})
		session.Subscribe(types.MarketTradeChannel, s.Symbol, types.SubscribeOptions{})
		// able to preload
		if s.MinInterval.Milliseconds() >= types.Interval1s.Milliseconds() && s.MinInterval.Milliseconds()%types.Interval1s.Milliseconds() == 0 {
			maxWindow := (s.Window + s.SmootherWindow + s.FisherTransformWindow) * (s.Interval.Milliseconds() / s.MinInterval.Milliseconds())
			bbgo.KLinePreloadLimit = int64((maxWindow/1000 + 1) * 1000)
			session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
				Interval: s.MinInterval,
			})
		} else {
			bbgo.KLinePreloadLimit = 0
		}
	} else {
		maxWindow := (s.Window + s.SmootherWindow + s.FisherTransformWindow) * (s.Interval.Milliseconds() / s.MinInterval.Milliseconds())
		bbgo.KLinePreloadLimit = int64((maxWindow/1000 + 1) * 1000)
		// gave up preload
		if s.Interval.Milliseconds() < s.MinInterval.Milliseconds() {
			bbgo.KLinePreloadLimit = 0
		}
		log.Errorf("set kLinePreloadLimit to %d, %d %d", bbgo.KLinePreloadLimit, s.Interval.Milliseconds()/s.MinInterval.Milliseconds(), maxWindow)
		if bbgo.KLinePreloadLimit > 0 {
			session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
				Interval: s.MinInterval,
			})
		}
	}
	s.ExitMethods.SetAndSubscribe(session, s)
}

func (s *Strategy) CurrentPosition() *types.Position {
	return s.Position
}

func (s *Strategy) SubmitOrder(ctx context.Context, submitOrder types.SubmitOrder) (*types.Order, error) {
	formattedOrder, err := s.Session.FormatOrder(submitOrder)
	if err != nil {
		return nil, err
	}
	createdOrders, errIdx, err := bbgo.BatchPlaceOrder(ctx, s.Session.Exchange, nil, formattedOrder)
	if len(errIdx) > 0 {
		return nil, err
	}
	return &createdOrders[0], err
}

const closeOrderRetryLimit = 5

func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	order := s.Position.NewMarketCloseOrder(percentage)
	if order == nil {
		return nil
	}
	order.Tag = "close"
	order.TimeInForce = ""

	order.MarginSideEffect = types.SideEffectTypeAutoRepay
	for i := 0; i < closeOrderRetryLimit; i++ {
		price := s.getLastPrice()
		balances := s.Session.GetAccount().Balances()
		baseBalance := balances[s.Market.BaseCurrency].Total()
		if order.Side == types.SideTypeBuy {
			quoteAmount := balances[s.Market.QuoteCurrency].Total().Div(price)
			if order.Quantity.Compare(quoteAmount) > 0 {
				order.Quantity = quoteAmount
			}
		} else if order.Side == types.SideTypeSell && order.Quantity.Compare(baseBalance) > 0 {
			order.Quantity = baseBalance
		}
		if s.Market.IsDustQuantity(order.Quantity, price) {
			return nil
		}
		o, err := s.SubmitOrder(ctx, *order)
		if err != nil {
			order.Quantity = order.Quantity.Mul(fixedpoint.One.Sub(Delta))
			continue
		}

		if o != nil {
			if o.Status == types.OrderStatusNew || o.Status == types.OrderStatusPartiallyFilled {
				log.Errorf("created Order when Close: %v", o)
			}
		}
		return nil
	}
	return errors.New("exceed retry limit")
}

func (s *Strategy) initIndicators(store *bbgo.SerialMarketDataStore) error {
	s.ma = &indicator.SMA{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.HLRangeWindow}}
	s.stdevHigh = &indicator.StdDev{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.HLRangeWindow}}
	s.stdevLow = &indicator.StdDev{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.HLRangeWindow}}
	s.drift = &DriftMA{
		drift: &indicator.WeightedDrift{
			MA:             &indicator.SMA{IntervalWindow: s.IntervalWindow},
			IntervalWindow: s.IntervalWindow,
		},
		ma1: &indicator.EWMA{
			IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.SmootherWindow},
		},
		ma2: &indicator.FisherTransform{
			IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.FisherTransformWindow},
		},
	}
	s.drift.SeriesBase.Series = s.drift
	s.atr = &indicator.ATR{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.ATRWindow}}
	s.trendLine = &indicator.EWMA{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.TrendWindow}}

	if bbgo.KLinePreloadLimit == 0 {
		return nil
	}
	klines, ok := store.KLinesOfInterval(s.Interval)
	klinesLength := len(*klines)
	if !ok || klinesLength == 0 {
		return errors.New("klines not exists")
	}
	log.Infof("loaded %d klines", klinesLength)
	for _, kline := range *klines {
		source := s.GetSource(&kline).Float64()
		high := kline.High.Float64()
		low := kline.Low.Float64()
		s.ma.Update(source)
		s.stdevHigh.Update(high - s.ma.Last(0))
		s.stdevLow.Update(s.ma.Last(0) - low)
		s.drift.Update(source, kline.Volume.Abs().Float64())
		s.trendLine.Update(source)
		s.atr.PushK(kline)
		s.priceLines.Update(source)
	}
	if s.frameKLine != nil && klines != nil {
		s.frameKLine.Set(&(*klines)[len(*klines)-1])
	}
	klines, ok = store.KLinesOfInterval(s.MinInterval)
	klinesLength = len(*klines)
	if !ok || klinesLength == 0 {
		return errors.New("klines not exists")
	}
	log.Infof("loaded %d klines%s", klinesLength, s.MinInterval)
	if s.klineMin != nil && klines != nil {
		s.klineMin.Set(&(*klines)[len(*klines)-1])
	}
	return nil
}

func (s *Strategy) smartCancel(ctx context.Context, pricef, atr float64, syscounter int) (int, error) {
	nonTraded := s.FastOrderExecutor.ActiveMakerOrders().Orders()
	if len(nonTraded) > 0 {
		if len(nonTraded) > 1 {
			log.Errorf("should only have one order to cancel, got %d", len(nonTraded))
		}
		toCancel := false

		for _, order := range nonTraded {
			if order.Status != types.OrderStatusNew && order.Status != types.OrderStatusPartiallyFilled {
				continue
			}
			s.pendingLock.Lock()
			counter := s.orderPendingCounter[order.OrderID]
			s.pendingLock.Unlock()

			log.Warnf("%v | counter: %d, system: %d", order, counter, syscounter)
			if syscounter-counter > s.PendingMinInterval {
				toCancel = true
			} else if order.Side == types.SideTypeBuy {
				// 75% of the probability
				if order.Price.Float64()+atr*2 <= pricef {
					toCancel = true
				}
			} else if order.Side == types.SideTypeSell {
				// 75% of the probability
				if order.Price.Float64()-atr*2 >= pricef {
					toCancel = true
				}
			} else {
				panic("not supported side for the order")
			}
		}
		if toCancel {
			err := s.FastOrderExecutor.Cancel(ctx)
			s.pendingLock.Lock()
			counters := s.orderPendingCounter
			s.orderPendingCounter = make(map[uint64]int)
			s.pendingLock.Unlock()
			// TODO: clean orderPendingCounter on cancel/trade
			for _, order := range nonTraded {
				counter := counters[order.OrderID]
				if order.Side == types.SideTypeSell {
					if s.maxCounterSellCanceled < counter {
						s.maxCounterSellCanceled = counter
					}
				} else {
					if s.maxCounterBuyCanceled < counter {
						s.maxCounterBuyCanceled = counter
					}
				}
			}
			log.Warnf("cancel all %v", err)
			return 0, err
		}
	}
	return len(nonTraded), nil
}

func (s *Strategy) trailingCheck(price float64, direction string) bool {
	if s.highestPrice > 0 && s.highestPrice < price {
		s.highestPrice = price
	}
	if s.lowestPrice > 0 && s.lowestPrice > price {
		s.lowestPrice = price
	}
	isShort := direction == "short"
	if isShort && s.sellPrice == 0 || !isShort && s.buyPrice == 0 {
		return false
	}
	for i := len(s.TrailingCallbackRate) - 1; i >= 0; i-- {
		trailingCallbackRate := s.TrailingCallbackRate[i]
		trailingActivationRatio := s.TrailingActivationRatio[i]
		if isShort {
			if (s.sellPrice-s.lowestPrice)/s.lowestPrice > trailingActivationRatio {
				return (price-s.lowestPrice)/s.lowestPrice > trailingCallbackRate
			}
		} else {
			if (s.highestPrice-s.buyPrice)/s.buyPrice > trailingActivationRatio {
				return (s.highestPrice-price)/s.buyPrice > trailingCallbackRate
			}
		}
	}
	return false
}

func (s *Strategy) initTickerFunctions(ctx context.Context) {
	if s.IsBackTesting() {
		s.getLastPrice = func() fixedpoint.Value {
			lastPrice, ok := s.Session.LastPrice(s.Symbol)
			if !ok {
				log.Error("cannot get lastprice")
			}
			return lastPrice
		}
	} else {
		s.Session.MarketDataStream.OnBookTickerUpdate(func(ticker types.BookTicker) {
			bestBid := ticker.Buy
			bestAsk := ticker.Sell

			if !util.TryLock(&s.lock) {
				return
			}
			if !bestAsk.IsZero() && !bestBid.IsZero() {
				s.midPrice = bestAsk.Add(bestBid).Div(Two)
			} else if !bestAsk.IsZero() {
				s.midPrice = bestAsk
			} else {
				s.midPrice = bestBid
			}
			s.lock.Unlock()

			// we removed realtime stoploss and trailingStop.

		})
		s.getLastPrice = func() (lastPrice fixedpoint.Value) {
			var ok bool
			s.lock.RLock()
			defer s.lock.RUnlock()
			if s.midPrice.IsZero() {
				lastPrice, ok = s.Session.LastPrice(s.Symbol)
				if !ok {
					log.Error("cannot get lastprice")
					return lastPrice
				}
			} else {
				lastPrice = s.midPrice
			}
			return lastPrice
		}
	}

}

// Sending new rebalance orders cost too much.
// Modify the position instead to expect the strategy itself rebalance on Close
func (s *Strategy) Rebalance(ctx context.Context) {
	price := s.getLastPrice()
	_, beta := types.LinearRegression(s.trendLine, 3)
	if math.Abs(beta) > s.RebalanceFilter && math.Abs(s.beta) > s.RebalanceFilter || math.Abs(s.beta) < s.RebalanceFilter && math.Abs(beta) < s.RebalanceFilter {
		return
	}
	balances := s.FastOrderExecutor.Session().GetAccount().Balances()
	baseBalance := balances[s.Market.BaseCurrency].Total()
	quoteBalance := balances[s.Market.QuoteCurrency].Total()
	total := baseBalance.Add(quoteBalance.Div(price))
	percentage := fixedpoint.One.Sub(Delta)
	log.Infof("rebalance beta %f %v", beta, s.Position)
	if beta > s.RebalanceFilter {
		if total.Mul(percentage).Compare(baseBalance) > 0 {
			q := total.Mul(percentage).Sub(baseBalance)
			s.Position.Lock()
			defer s.Position.Unlock()
			s.Position.Base = q.Neg()
			s.Position.Quote = q.Mul(price)
			s.Position.AverageCost = price
		}
	} else if beta <= -s.RebalanceFilter {
		if total.Mul(percentage).Compare(quoteBalance.Div(price)) > 0 {
			q := total.Mul(percentage).Sub(quoteBalance.Div(price))
			s.Position.Lock()
			defer s.Position.Unlock()
			s.Position.Base = q
			s.Position.Quote = q.Mul(price).Neg()
			s.Position.AverageCost = price
		}
	} else {
		if total.Div(Two).Compare(quoteBalance.Div(price)) > 0 {
			q := total.Div(Two).Sub(quoteBalance.Div(price))
			s.Position.Lock()
			defer s.Position.Unlock()
			s.Position.Base = q
			s.Position.Quote = q.Mul(price).Neg()
			s.Position.AverageCost = price
		} else if total.Div(Two).Compare(baseBalance) > 0 {
			q := total.Div(Two).Sub(baseBalance)
			s.Position.Lock()
			defer s.Position.Unlock()
			s.Position.Base = q.Neg()
			s.Position.Quote = q.Mul(price)
			s.Position.AverageCost = price
		} else {
			s.Position.Lock()
			defer s.Position.Unlock()
			s.Position.Reset()
		}
	}
	log.Infof("rebalanceafter %v %v %v", baseBalance, quoteBalance, s.Position)
	s.beta = beta
}

func (s *Strategy) CalcAssetValue(price fixedpoint.Value) fixedpoint.Value {
	balances := s.Session.GetAccount().Balances()
	return balances[s.Market.BaseCurrency].Total().Mul(price).Add(balances[s.Market.QuoteCurrency].Total())
}

func (s *Strategy) klineHandlerMin(ctx context.Context, kline types.KLine, counter int) {
	s.klineMin.Set(&kline)
	if s.Status != types.StrategyStatusRunning {
		return
	}
	// for doing the trailing stoploss during backtesting
	atr := s.atr.Last(0)
	price := s.getLastPrice()
	pricef := price.Float64()

	lowf := math.Min(kline.Low.Float64(), pricef)
	highf := math.Max(kline.High.Float64(), pricef)
	s.positionLock.Lock()
	if s.lowestPrice > 0 && lowf < s.lowestPrice {
		s.lowestPrice = lowf
	}
	if s.highestPrice > 0 && highf > s.highestPrice {
		s.highestPrice = highf
	}
	s.positionLock.Unlock()

	numPending := 0
	var err error
	if numPending, err = s.smartCancel(ctx, pricef, atr, counter); err != nil {
		log.WithError(err).Errorf("cannot cancel orders")
		return
	}
	if numPending > 0 {
		return
	}

	if s.NoTrailingStopLoss {
		return
	}

	exitCondition := s.CheckStopLoss() || s.trailingCheck(highf, "short") || s.trailingCheck(lowf, "long")
	if exitCondition {
		_ = s.ClosePosition(ctx, fixedpoint.One)
	}
}

func (s *Strategy) klineHandler(ctx context.Context, kline types.KLine, counter int) {
	start := time.Now()
	defer func() {
		end := time.Now()
		elapsed := end.Sub(start)
		s.elapsed.Update(float64(elapsed) / 1000000)
	}()
	s.frameKLine.Set(&kline)

	source := s.GetSource(&kline)
	sourcef := source.Float64()

	s.priceLines.Update(sourcef)
	s.ma.Update(sourcef)
	s.trendLine.Update(sourcef)

	s.drift.Update(sourcef, kline.Volume.Abs().Float64())
	s.atr.PushK(kline)
	atr := s.atr.Last(0)

	price := kline.Close // s.getLastPrice()
	pricef := price.Float64()
	lowf := math.Min(kline.Low.Float64(), pricef)
	highf := math.Max(kline.High.Float64(), pricef)
	lowdiff := pricef - lowf
	s.stdevLow.Update(lowdiff)
	highdiff := highf - pricef
	s.stdevHigh.Update(highdiff)

	drift := s.drift.Array(2)

	if len(drift) < 2 || len(drift) < s.PredictOffset {
		return
	}
	ddrift := s.drift.drift.Array(2)
	if len(ddrift) < 2 || len(ddrift) < s.PredictOffset {
		return
	}

	if s.Status != types.StrategyStatusRunning {
		return
	}

	log.Infof("highdiff: %3.2f open: %8v, close: %8v, high: %8v, low: %8v, time: %v %v", s.stdevHigh.Last(0), kline.Open, kline.Close, kline.High, kline.Low, kline.StartTime, kline.EndTime)

	s.positionLock.Lock()
	if s.lowestPrice > 0 && lowf < s.lowestPrice {
		s.lowestPrice = lowf
	}
	if s.highestPrice > 0 && highf > s.highestPrice {
		s.highestPrice = highf
	}
	s.positionLock.Unlock()

	if !s.NoRebalance {
		s.Rebalance(ctx)
	}

	if s.Debug {
		balances := s.FastOrderExecutor.Session().GetAccount().Balances()
		bbgo.Notify("source: %.4f, price: %.4f, drift[0]: %.4f, ddrift[0]: %.4f, lowf %.4f, highf: %.4f lowest: %.4f highest: %.4f sp %.4f bp %.4f",
			sourcef, pricef, drift[0], ddrift[0], atr, lowf, highf, s.lowestPrice, s.highestPrice, s.sellPrice, s.buyPrice)
		// Notify will parse args to strings and process separately
		bbgo.Notify("balances: [Total] %v %s [Base] %s(%v %s) [Quote] %s",
			s.CalcAssetValue(price),
			s.Market.QuoteCurrency,
			balances[s.Market.BaseCurrency].String(),
			balances[s.Market.BaseCurrency].Total().Mul(price),
			s.Market.QuoteCurrency,
			balances[s.Market.QuoteCurrency].String(),
		)
	}

	shortCondition := drift[1] >= 0 && drift[0] <= 0 || (drift[1] >= drift[0] && drift[1] <= 0) || ddrift[1] >= 0 && ddrift[0] <= 0 || (ddrift[1] >= ddrift[0] && ddrift[1] <= 0)
	longCondition := drift[1] <= 0 && drift[0] >= 0 || (drift[1] <= drift[0] && drift[1] >= 0) || ddrift[1] <= 0 && ddrift[0] >= 0 || (ddrift[1] <= ddrift[0] && ddrift[1] >= 0)
	if shortCondition && longCondition {
		if s.priceLines.Index(1) > s.priceLines.Last(0) {
			longCondition = false
		} else {
			shortCondition = false
		}
	}
	exitCondition := !s.NoTrailingStopLoss && (s.CheckStopLoss() || s.trailingCheck(pricef, "short") || s.trailingCheck(pricef, "long"))

	if exitCondition || longCondition || shortCondition {
		var err error
		var hold int
		if hold, err = s.smartCancel(ctx, pricef, atr, counter); err != nil {
			log.WithError(err).Errorf("cannot cancel orders")
		}
		if hold > 0 {
			return
		}
	} else {
		if _, err := s.smartCancel(ctx, pricef, atr, counter); err != nil {
			log.WithError(err).Errorf("cannot cancel orders")
		}
		return
	}
	if exitCondition {
		_ = s.ClosePosition(ctx, fixedpoint.One)
		return
	}

	if longCondition {
		source = source.Sub(fixedpoint.NewFromFloat(s.stdevLow.Last(0) * s.HighLowVarianceMultiplier))
		if source.Compare(price) > 0 {
			source = price
		}

		opt := s.OpenPositionOptions
		opt.Long = true
		opt.LimitOrder = true
		// force to use market taker
		if counter-s.maxCounterBuyCanceled <= s.PendingMinInterval && s.maxCounterBuyCanceled > s.maxCounterSellCanceled {
			opt.LimitOrder = false
			source = price
		}
		opt.Price = source
		opt.Tags = []string{"long"}

		submitOrder, err := s.FastOrderExecutor.NewOrderFromOpenPosition(ctx, &opt)
		if err != nil {
			errs := filterErrors(multierr.Errors(err))
			if len(errs) > 0 {
				log.Errorf("%v", errs)
				log.WithError(err).Errorf("cannot place buy order")
			}
			return
		}
		if submitOrder == nil {
			return
		}

		log.Infof("source in long %v %v %f", source, price, s.stdevLow.Last(0))

		o, err := s.SubmitOrder(ctx, *submitOrder)
		if err != nil {
			log.WithError(err).Errorf("cannot place buy order")
			return
		}

		log.Infof("order %v", o)
		if o != nil {
			if o.Status == types.OrderStatusNew || o.Status == types.OrderStatusPartiallyFilled {
				s.pendingLock.Lock()
				if _, ok := s.orderPendingCounter[o.OrderID]; !ok {
					s.orderPendingCounter[o.OrderID] = counter
				}
				s.pendingLock.Unlock()
			}
		}
		return
	}
	if shortCondition {
		source = source.Add(fixedpoint.NewFromFloat(s.stdevHigh.Last(0) * s.HighLowVarianceMultiplier))
		if source.Compare(price) < 0 {
			source = price
		}

		log.Infof("source in short: %v %v %f", source, price, s.stdevLow.Last(0))

		opt := s.OpenPositionOptions
		opt.Short = true
		opt.LimitOrder = true
		if counter-s.maxCounterSellCanceled <= s.PendingMinInterval && s.maxCounterSellCanceled > s.maxCounterBuyCanceled {
			opt.LimitOrder = false
			source = price
		}
		opt.Price = source
		opt.Tags = []string{"short"}
		submitOrder, err := s.FastOrderExecutor.NewOrderFromOpenPosition(ctx, &opt)
		if err != nil {
			errs := filterErrors(multierr.Errors(err))
			if len(errs) > 0 {
				log.WithError(err).Errorf("cannot place sell order")
			}
			return
		}
		if submitOrder == nil {
			return
		}
		o, err := s.SubmitOrder(ctx, *submitOrder)
		if err != nil {
			log.WithError(err).Errorf("cannot place sell order")
			return
		}
		log.Infof("order %v", o)
		if o != nil {
			if o.Status == types.OrderStatusNew || o.Status == types.OrderStatusPartiallyFilled {
				s.pendingLock.Lock()
				if _, ok := s.orderPendingCounter[o.OrderID]; !ok {
					s.orderPendingCounter[o.OrderID] = counter
				}
				s.pendingLock.Unlock()
			}
		}
		return
	}
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.Leverage == fixedpoint.Zero {
		s.Leverage = fixedpoint.One
	}
	instanceID := s.InstanceID()
	// Will be set by persistence if there's any from DB
	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}
	if s.Session.MakerFeeRate.Sign() > 0 || s.Session.TakerFeeRate.Sign() > 0 {
		s.Position.SetExchangeFeeRate(s.Session.ExchangeName, types.ExchangeFee{
			MakerFeeRate: s.Session.MakerFeeRate,
			TakerFeeRate: s.Session.TakerFeeRate,
		})
	}
	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}
	if s.TradeStats == nil {
		s.TradeStats = types.NewTradeStats(s.Symbol)
	}
	// StrategyController
	s.Status = types.StrategyStatusRunning

	s.OnSuspend(func() {
		_ = s.FastOrderExecutor.GracefulCancel(ctx)
	})

	s.OnEmergencyStop(func() {
		_ = s.FastOrderExecutor.GracefulCancel(ctx)
		_ = s.ClosePosition(ctx, fixedpoint.One)
	})

	profitChart := floats.Slice{1., 1.}
	price, _ := s.Session.LastPrice(s.Symbol)
	initAsset := s.CalcAssetValue(price).Float64()
	cumProfit := floats.Slice{initAsset, initAsset}
	modify := func(p float64) float64 {
		return p
	}
	if s.GraphPNLDeductFee {
		modify = func(p float64) float64 {
			return p * (1. - Fee)
		}
	}

	s.FastOrderExecutor = bbgo.NewFastOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.FastOrderExecutor.DisableNotify()
	orderStore := s.FastOrderExecutor.OrderStore()
	orderStore.AddOrderUpdate = true
	orderStore.RemoveCancelled = true
	orderStore.RemoveFilled = true
	activeOrders := s.FastOrderExecutor.ActiveMakerOrders()
	tradeCollector := s.FastOrderExecutor.TradeCollector()
	tradeStore := tradeCollector.TradeStore()

	syscounter := 0

	// Modify activeOrders to force write order updates
	s.Session.UserDataStream.OnOrderUpdate(func(order types.Order) {
		hasSymbol := len(activeOrders.Symbol) > 0
		if hasSymbol && order.Symbol != activeOrders.Symbol {
			return
		}

		switch order.Status {
		case types.OrderStatusFilled:
			s.pendingLock.Lock()
			s.orderPendingCounter = make(map[uint64]int)
			s.pendingLock.Unlock()
			// make sure we have the order and we remove it
			activeOrders.Remove(order)

		case types.OrderStatusPartiallyFilled:
			s.pendingLock.Lock()
			if _, ok := s.orderPendingCounter[order.OrderID]; !ok {
				s.orderPendingCounter[order.OrderID] = syscounter
			}
			s.pendingLock.Unlock()
			activeOrders.Add(order)

		case types.OrderStatusNew:
			s.pendingLock.Lock()
			if _, ok := s.orderPendingCounter[order.OrderID]; !ok {
				s.orderPendingCounter[order.OrderID] = syscounter
			}
			s.pendingLock.Unlock()
			activeOrders.Add(order)

		case types.OrderStatusCanceled, types.OrderStatusRejected:
			log.Debugf("[ActiveOrderBook] order status %s, removing order %s", order.Status, order)
			s.pendingLock.Lock()
			s.orderPendingCounter = make(map[uint64]int)
			s.pendingLock.Unlock()
			activeOrders.Remove(order)

		default:
			log.Errorf("unhandled order status: %s", order.Status)
		}
		orderStore.HandleOrderUpdate(order)
	})
	s.Session.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
		if trade.Symbol != s.Symbol {
			return
		}
		profit, netProfit, madeProfit := s.Position.AddTrade(trade)
		tradeStore.Add(trade)
		if madeProfit {
			p := s.Position.NewProfit(trade, profit, netProfit)
			s.Environment.RecordPosition(s.Position, trade, &p)
			s.TradeStats.Add(&p)
			s.ProfitStats.AddTrade(trade)
			s.ProfitStats.AddProfit(p)
			bbgo.Notify(&p)
			bbgo.Notify(s.ProfitStats)
		}

		price := trade.Price.Float64()

		if s.buyPrice > 0 {
			profitChart.Update(modify(price / s.buyPrice))
			cumProfit.Update(s.CalcAssetValue(trade.Price).Float64())
		} else if s.sellPrice > 0 {
			profitChart.Update(modify(s.sellPrice / price))
			cumProfit.Update(s.CalcAssetValue(trade.Price).Float64())
		}
		s.positionLock.Lock()
		if s.Position.IsDust(trade.Price) {
			s.buyPrice = 0
			s.sellPrice = 0
			s.highestPrice = 0
			s.lowestPrice = 0
		} else if s.Position.IsLong() {
			s.buyPrice = s.Position.ApproximateAverageCost.Float64()
			s.sellPrice = 0
			s.highestPrice = math.Max(s.buyPrice, s.highestPrice)
			s.lowestPrice = s.buyPrice
		} else if s.Position.IsShort() {
			s.sellPrice = s.Position.ApproximateAverageCost.Float64()
			s.buyPrice = 0
			s.highestPrice = s.sellPrice
			if s.lowestPrice == 0 {
				s.lowestPrice = s.sellPrice
			} else {
				s.lowestPrice = math.Min(s.lowestPrice, s.sellPrice)
			}
		}
		s.positionLock.Unlock()
	})

	s.orderPendingCounter = make(map[uint64]int)

	// Exit methods from config
	for _, method := range s.ExitMethods {
		method.Bind(session, s.GeneralOrderExecutor)
	}

	s.frameKLine = &types.KLine{}
	s.klineMin = &types.KLine{}
	s.priceLines = types.NewQueue(300)
	s.elapsed = types.NewQueue(60000)

	s.initTickerFunctions(ctx)
	s.startTime = s.Environment.StartTime()
	s.TradeStats.SetIntervalProfitCollector(types.NewIntervalProfitCollector(types.Interval1d, s.startTime))
	s.TradeStats.SetIntervalProfitCollector(types.NewIntervalProfitCollector(types.Interval1w, s.startTime))

	s.InitDrawCommands(&profitChart, &cumProfit)

	bbgo.RegisterCommand("/config", "Show latest config", func(reply interact.Reply) {
		var buffer bytes.Buffer
		s.Print(&buffer, false)
		reply.Message(buffer.String())
	})

	bbgo.RegisterCommand("/dump", "Dump internal params", func(reply interact.Reply) {
		reply.Message("Please enter series output length:")
	}).Next(func(length string, reply interact.Reply) {
		var buffer bytes.Buffer
		l, err := strconv.Atoi(length)
		if err != nil {
			dynamic.ParamDump(s, &buffer)
		} else {
			dynamic.ParamDump(s, &buffer, l)
		}
		reply.Message(buffer.String())
	})

	bbgo.RegisterModifier(s)

	// event trigger order: s.Interval => s.MinInterval
	store, ok := session.SerialMarketDataStore(ctx, s.Symbol, []types.Interval{s.Interval, s.MinInterval}, !bbgo.IsBackTesting)
	if !ok {
		panic("cannot get " + s.MinInterval + " history")
	}
	if err := s.initIndicators(store); err != nil {
		log.WithError(err).Errorf("initIndicator failed")
		return nil
	}

	store.OnKLineClosed(func(kline types.KLine) {
		counter := int(kline.StartTime.Time().Add(kline.Interval.Duration()).Sub(s.startTime).Milliseconds()) / s.MinInterval.Milliseconds()
		syscounter = counter
		if kline.Interval == s.Interval {
			s.klineHandler(ctx, kline, counter)
		} else if kline.Interval == s.MinInterval {
			s.klineHandlerMin(ctx, kline, counter)
		}
	})

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {

		if !bbgo.IsBackTesting {
			bbgo.Sync(ctx, s)
		}

		var buffer bytes.Buffer

		s.Print(&buffer, true, true)

		fmt.Fprintln(&buffer, "--- NonProfitable Dates ---")
		for _, daypnl := range s.TradeStats.IntervalProfits[types.Interval1d].GetNonProfitableIntervals() {
			fmt.Fprintf(&buffer, "%s\n", daypnl)
		}
		fmt.Fprintln(&buffer, s.TradeStats.BriefString())

		fmt.Fprintf(&buffer, "%v\n", s.orderPendingCounter)

		os.Stdout.Write(buffer.Bytes())

		if s.GenerateGraph {
			s.Draw(s.frameKLine.StartTime, &profitChart, &cumProfit)
		}
		wg.Done()
	})
	return nil
}
