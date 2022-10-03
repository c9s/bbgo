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
	"github.com/wcharczuk/go-chart/v2"
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

	p *types.Position

	priceLines          *types.Queue
	trendLine           types.UpdatableSeriesExtend
	ma                  types.UpdatableSeriesExtend
	stdevHigh           *indicator.StdDev
	stdevLow            *indicator.StdDev
	drift               *DriftMA
	atr                 *indicator.ATR
	midPrice            fixedpoint.Value
	lock                sync.RWMutex `ignore:"true"`
	positionLock        sync.RWMutex `ignore:"true"`
	startTime           time.Time
	minutesCounter      int
	orderPendingCounter map[uint64]int
	frameKLine          *types.KLine
	kline1m             *types.KLine

	beta float64

	UseStopLoss               bool             `json:"useStopLoss" modifiable:"true"`
	UseAtr                    bool             `json:"useAtr" modifiable:"true"`
	StopLoss                  fixedpoint.Value `json:"stoploss" modifiable:"true"`
	CanvasPath                string           `json:"canvasPath"`
	PredictOffset             int              `json:"predictOffset"`
	HighLowVarianceMultiplier float64          `json:"hlVarianceMultiplier" modifiable:"true"`
	NoTrailingStopLoss        bool             `json:"noTrailingStopLoss" modifiable:"true"`
	TrailingStopLossType      string           `json:"trailingStopLossType" modifiable:"true"` // trailing stop sources. Possible options are `kline` for 1m kline and `realtime` from order updates
	HLRangeWindow             int              `json:"hlRangeWindow"`
	SmootherWindow            int              `json:"smootherWindow"`
	FisherTransformWindow     int              `json:"fisherTransformWindow"`
	ATRWindow                 int              `json:"atrWindow"`
	PendingMinutes            int              `json:"pendingMinutes" modifiable:"true"`  // if order not be traded for pendingMinutes of time, cancel it.
	NoRebalance               bool             `json:"noRebalance" modifiable:"true"`     // disable rebalance
	TrendWindow               int              `json:"trendWindow"`                       // trendLine is used for rebalancing the position. When trendLine goes up, hold base, otherwise hold quote
	RebalanceFilter           float64          `json:"rebalanceFilter" modifiable:"true"` // beta filter on the Linear Regression of trendLine
	TrailingCallbackRate      []float64        `json:"trailingCallbackRate" modifiable:"true"`
	TrailingActivationRatio   []float64        `json:"trailingActivationRatio" modifiable:"true"`

	buyPrice     float64 `persistence:"buy_price"`
	sellPrice    float64 `persistence:"sell_price"`
	highestPrice float64 `persistence:"highest_price"`
	lowestPrice  float64 `persistence:"lowest_price"`

	// This is not related to trade but for statistics graph generation
	// Will deduct fee in percentage from every trade
	GraphPNLDeductFee bool   `json:"graphPNLDeductFee"`
	GraphPNLPath      string `json:"graphPNLPath"`
	GraphCumPNLPath   string `json:"graphCumPNLPath"`
	// Whether to generate graph when shutdown
	GenerateGraph bool `json:"generateGraph"`

	ExitMethods bbgo.ExitMethodSet `json:"exits"`
	Session     *bbgo.ExchangeSession
	*bbgo.GeneralOrderExecutor

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
	maxWindow := (s.Window + s.SmootherWindow + s.FisherTransformWindow) * s.Interval.Minutes()
	bbgo.KLinePreloadLimit = int64((maxWindow/1000 + 1) * 1000)
	log.Errorf("set kLinePreloadLimit to %d, %d %d", bbgo.KLinePreloadLimit, s.Interval.Minutes(), maxWindow)
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
		Interval: types.Interval1m,
	})

	if !bbgo.IsBackTesting {
		session.Subscribe(types.BookTickerChannel, s.Symbol, types.SubscribeOptions{})
	}
	s.ExitMethods.SetAndSubscribe(session, s)
}

func (s *Strategy) CurrentPosition() *types.Position {
	return s.Position
}

const closeOrderRetryLimit = 5

func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	order := s.p.NewMarketCloseOrder(percentage)
	if order == nil {
		return nil
	}
	order.Tag = "close"
	order.TimeInForce = ""

	order.MarginSideEffect = types.SideEffectTypeAutoRepay
	for i := 0; i < closeOrderRetryLimit; i++ {
		price := s.getLastPrice()
		balances := s.GeneralOrderExecutor.Session().GetAccount().Balances()
		baseBalance := balances[s.Market.BaseCurrency].Available
		if order.Side == types.SideTypeBuy {
			quoteAmount := balances[s.Market.QuoteCurrency].Available.Div(price)
			if order.Quantity.Compare(quoteAmount) > 0 {
				order.Quantity = quoteAmount
			}
		} else if order.Side == types.SideTypeSell && order.Quantity.Compare(baseBalance) > 0 {
			order.Quantity = baseBalance
		}
		if s.Market.IsDustQuantity(order.Quantity, price) {
			return nil
		}
		_, err := s.GeneralOrderExecutor.SubmitOrders(ctx, *order)
		if err != nil {
			order.Quantity = order.Quantity.Mul(fixedpoint.One.Sub(Delta))
			continue
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
		s.stdevHigh.Update(high - s.ma.Last())
		s.stdevLow.Update(s.ma.Last() - low)
		s.drift.Update(source, kline.Volume.Abs().Float64())
		s.trendLine.Update(source)
		s.atr.PushK(kline)
		s.priceLines.Update(source)
	}
	if s.frameKLine != nil && klines != nil {
		s.frameKLine.Set(&(*klines)[len(*klines)-1])
	}
	klines, ok = store.KLinesOfInterval(types.Interval1m)
	klinesLength = len(*klines)
	if !ok || klinesLength == 0 {
		return errors.New("klines not exists")
	}
	log.Infof("loaded %d klines1m", klinesLength)
	if s.kline1m != nil && klines != nil {
		s.kline1m.Set(&(*klines)[len(*klines)-1])
	}
	s.startTime = s.kline1m.StartTime.Time().Add(s.kline1m.Interval.Duration())
	return nil
}

func (s *Strategy) smartCancel(ctx context.Context, pricef, atr float64) (int, error) {
	nonTraded := s.GeneralOrderExecutor.ActiveMakerOrders().Orders()
	if len(nonTraded) > 0 {
		if len(nonTraded) > 1 {
			log.Errorf("should only have one order to cancel, got %d", len(nonTraded))
		}
		toCancel := false

		for _, order := range nonTraded {
			if order.Status != types.OrderStatusNew && order.Status != types.OrderStatusPartiallyFilled {
				continue
			}
			log.Warnf("%v | counter: %d, system: %d", order, s.orderPendingCounter[order.OrderID], s.minutesCounter)
			if s.minutesCounter-s.orderPendingCounter[order.OrderID] > s.PendingMinutes {
				toCancel = true
			} else if order.Side == types.SideTypeBuy {
				// 75% of the probability
				if order.Price.Float64()+s.stdevHigh.Last()*2 <= pricef {
					toCancel = true
				}
			} else if order.Side == types.SideTypeSell {
				// 75% of the probability
				if order.Price.Float64()-s.stdevLow.Last()*2 >= pricef {
					toCancel = true
				}
			} else {
				panic("not supported side for the order")
			}
		}
		if toCancel {
			err := s.GeneralOrderExecutor.GracefulCancel(ctx)
			// TODO: clean orderPendingCounter on cancel/trade
			for _, order := range nonTraded {
				delete(s.orderPendingCounter, order.OrderID)
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

			var pricef float64
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
			pricef = s.midPrice.Float64()

			s.lock.Unlock()

			if !util.TryLock(&s.positionLock) {
				return
			}

			if s.highestPrice > 0 && s.highestPrice < pricef {
				s.highestPrice = pricef
			}
			if s.lowestPrice > 0 && s.lowestPrice > pricef {
				s.lowestPrice = pricef
			}
			if s.CheckStopLoss() {
				s.positionLock.Unlock()
				s.ClosePosition(ctx, fixedpoint.One)
				return
			}
			// for trailing stoploss during the realtime
			if s.NoTrailingStopLoss || s.TrailingStopLossType == "kline" {
				s.positionLock.Unlock()
				return
			}

			exitCondition := s.trailingCheck(pricef, "short") || s.trailingCheck(pricef, "long")

			s.positionLock.Unlock()
			if exitCondition {
				s.ClosePosition(ctx, fixedpoint.One)
			}
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

func (s *Strategy) DrawIndicators(time types.Time) *types.Canvas {
	canvas := types.NewCanvas(s.InstanceID(), s.Interval)
	Length := s.priceLines.Length()
	if Length > 300 {
		Length = 300
	}
	log.Infof("draw indicators with %d data", Length)
	mean := s.priceLines.Mean(Length)
	highestPrice := s.priceLines.Minus(mean).Abs().Highest(Length)
	highestDrift := s.drift.Abs().Highest(Length)
	hi := s.drift.drift.Abs().Highest(Length)
	ratio := highestPrice / highestDrift

	// canvas.Plot("upband", s.ma.Add(s.stdevHigh), time, Length)
	canvas.Plot("ma", s.ma, time, Length)
	// canvas.Plot("downband", s.ma.Minus(s.stdevLow), time, Length)
	fmt.Printf("%f %f\n", highestPrice, hi)

	canvas.Plot("trend", s.trendLine, time, Length)
	canvas.Plot("drift", s.drift.Mul(ratio).Add(mean), time, Length)
	canvas.Plot("driftOrig", s.drift.drift.Mul(highestPrice/hi).Add(mean), time, Length)
	canvas.Plot("zero", types.NumberSeries(mean), time, Length)
	canvas.Plot("price", s.priceLines, time, Length)
	return canvas
}

func (s *Strategy) DrawPNL(profit types.Series) *types.Canvas {
	canvas := types.NewCanvas(s.InstanceID())
	log.Errorf("pnl Highest: %f, Lowest: %f", types.Highest(profit, profit.Length()), types.Lowest(profit, profit.Length()))
	length := profit.Length()
	if s.GraphPNLDeductFee {
		canvas.PlotRaw("pnl % (with Fee Deducted)", profit, length)
	} else {
		canvas.PlotRaw("pnl %", profit, length)
	}
	canvas.YAxis = chart.YAxis{
		ValueFormatter: func(v interface{}) string {
			if vf, isFloat := v.(float64); isFloat {
				return fmt.Sprintf("%.4f", vf)
			}
			return ""
		},
	}
	canvas.PlotRaw("1", types.NumberSeries(1), length)
	return canvas
}

func (s *Strategy) DrawCumPNL(cumProfit types.Series) *types.Canvas {
	canvas := types.NewCanvas(s.InstanceID())
	canvas.PlotRaw("cummulative pnl", cumProfit, cumProfit.Length())
	canvas.YAxis = chart.YAxis{
		ValueFormatter: func(v interface{}) string {
			if vf, isFloat := v.(float64); isFloat {
				return fmt.Sprintf("%.4f", vf)
			}
			return ""
		},
	}
	return canvas
}

func (s *Strategy) Draw(time types.Time, profit types.Series, cumProfit types.Series) {
	canvas := s.DrawIndicators(time)
	f, err := os.Create(s.CanvasPath)
	if err != nil {
		log.WithError(err).Errorf("cannot create on %s", s.CanvasPath)
		return
	}
	defer f.Close()
	if err := canvas.Render(chart.PNG, f); err != nil {
		log.WithError(err).Errorf("cannot render in drift")
	}

	canvas = s.DrawPNL(profit)
	f, err = os.Create(s.GraphPNLPath)
	if err != nil {
		log.WithError(err).Errorf("open pnl")
		return
	}
	defer f.Close()
	if err := canvas.Render(chart.PNG, f); err != nil {
		log.WithError(err).Errorf("render pnl")
	}

	canvas = s.DrawCumPNL(cumProfit)
	f, err = os.Create(s.GraphCumPNLPath)
	if err != nil {
		log.WithError(err).Errorf("open cumpnl")
		return
	}
	defer f.Close()
	if err := canvas.Render(chart.PNG, f); err != nil {
		log.WithError(err).Errorf("render cumpnl")
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
	balances := s.GeneralOrderExecutor.Session().GetAccount().Balances()
	baseBalance := balances[s.Market.BaseCurrency].Total()
	quoteBalance := balances[s.Market.QuoteCurrency].Total()
	total := baseBalance.Add(quoteBalance.Div(price))
	percentage := fixedpoint.One.Sub(Delta)
	log.Infof("rebalance beta %f %v", beta, s.p)
	if beta > s.RebalanceFilter {
		if total.Mul(percentage).Compare(baseBalance) > 0 {
			q := total.Mul(percentage).Sub(baseBalance)
			s.p.Lock()
			defer s.p.Unlock()
			s.p.Base = q.Neg()
			s.p.Quote = q.Mul(price)
			s.p.AverageCost = price
		}
	} else if beta <= -s.RebalanceFilter {
		if total.Mul(percentage).Compare(quoteBalance.Div(price)) > 0 {
			q := total.Mul(percentage).Sub(quoteBalance.Div(price))
			s.p.Lock()
			defer s.p.Unlock()
			s.p.Base = q
			s.p.Quote = q.Mul(price).Neg()
			s.p.AverageCost = price
		}
	} else {
		if total.Div(Two).Compare(quoteBalance.Div(price)) > 0 {
			q := total.Div(Two).Sub(quoteBalance.Div(price))
			s.p.Lock()
			defer s.p.Unlock()
			s.p.Base = q
			s.p.Quote = q.Mul(price).Neg()
			s.p.AverageCost = price
		} else if total.Div(Two).Compare(baseBalance) > 0 {
			q := total.Div(Two).Sub(baseBalance)
			s.p.Lock()
			defer s.p.Unlock()
			s.p.Base = q.Neg()
			s.p.Quote = q.Mul(price)
			s.p.AverageCost = price
		} else {
			s.p.Lock()
			defer s.p.Unlock()
			s.p.Reset()
		}
	}
	log.Infof("rebalanceafter %v %v %v", baseBalance, quoteBalance, s.p)
	s.beta = beta
}

func (s *Strategy) CalcAssetValue(price fixedpoint.Value) fixedpoint.Value {
	balances := s.Session.GetAccount().Balances()
	return balances[s.Market.BaseCurrency].Total().Mul(price).Add(balances[s.Market.QuoteCurrency].Total())
}

func (s *Strategy) klineHandler1m(ctx context.Context, kline types.KLine) {
	s.kline1m.Set(&kline)
	if s.Status != types.StrategyStatusRunning {
		return
	}
	// for doing the trailing stoploss during backtesting
	atr := s.atr.Last()
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

	numPending := 0
	var err error
	if numPending, err = s.smartCancel(ctx, pricef, atr); err != nil {
		log.WithError(err).Errorf("cannot cancel orders")
		s.positionLock.Unlock()
		return
	}
	if numPending > 0 {
		s.positionLock.Unlock()
		return
	}

	if s.NoTrailingStopLoss || s.TrailingStopLossType == "realtime" {
		s.positionLock.Unlock()
		return
	}

	exitCondition := s.CheckStopLoss() || s.trailingCheck(highf, "short") || s.trailingCheck(lowf, "long")
	s.positionLock.Unlock()
	if exitCondition {
		_ = s.ClosePosition(ctx, fixedpoint.One)
	}
}

func (s *Strategy) klineHandler(ctx context.Context, kline types.KLine) {
	var driftPred, atr float64
	var drift []float64

	s.frameKLine.Set(&kline)

	source := s.GetSource(&kline)
	sourcef := source.Float64()
	s.priceLines.Update(sourcef)
	s.ma.Update(sourcef)
	s.trendLine.Update(sourcef)
	s.drift.Update(sourcef, kline.Volume.Abs().Float64())

	s.atr.PushK(kline)

	driftPred = s.drift.Predict(s.PredictOffset)
	ddriftPred := s.drift.drift.Predict(s.PredictOffset)
	atr = s.atr.Last()
	price := s.getLastPrice()
	pricef := price.Float64()
	lowf := math.Min(kline.Low.Float64(), pricef)
	highf := math.Max(kline.High.Float64(), pricef)
	lowdiff := s.ma.Last() - lowf
	s.stdevLow.Update(lowdiff)
	highdiff := highf - s.ma.Last()
	s.stdevHigh.Update(highdiff)
	drift = s.drift.Array(2)
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

	s.positionLock.Lock()
	log.Infof("highdiff: %3.2f ma: %.2f, open: %8v, close: %8v, high: %8v, low: %8v, time: %v %v", s.stdevHigh.Last(), s.ma.Last(), kline.Open, kline.Close, kline.High, kline.Low, kline.StartTime, kline.EndTime)
	if s.lowestPrice > 0 && lowf < s.lowestPrice {
		s.lowestPrice = lowf
	}
	if s.highestPrice > 0 && highf > s.highestPrice {
		s.highestPrice = highf
	}

	if !s.NoRebalance {
		s.Rebalance(ctx)
	}

	balances := s.GeneralOrderExecutor.Session().GetAccount().Balances()
	bbgo.Notify("source: %.4f, price: %.4f, driftPred: %.4f, ddriftPred: %.4f, drift[1]: %.4f, ddrift[1]: %.4f, atr: %.4f, lowf %.4f, highf: %.4f lowest: %.4f highest: %.4f sp %.4f bp %.4f",
		sourcef, pricef, driftPred, ddriftPred, drift[1], ddrift[1], atr, lowf, highf, s.lowestPrice, s.highestPrice, s.sellPrice, s.buyPrice)
	// Notify will parse args to strings and process separately
	bbgo.Notify("balances: [Total] %v %s [Base] %s(%v %s) [Quote] %s",
		s.CalcAssetValue(price),
		s.Market.QuoteCurrency,
		balances[s.Market.BaseCurrency].String(),
		balances[s.Market.BaseCurrency].Total().Mul(price),
		s.Market.QuoteCurrency,
		balances[s.Market.QuoteCurrency].String(),
	)

	shortCondition := drift[1] >= 0 && drift[0] <= 0 || (drift[1] >= drift[0] && drift[1] <= 0) || ddrift[1] >= 0 && ddrift[0] <= 0 || (ddrift[1] >= ddrift[0] && ddrift[1] <= 0)
	longCondition := drift[1] <= 0 && drift[0] >= 0 || (drift[1] <= drift[0] && drift[1] >= 0) || ddrift[1] <= 0 && ddrift[0] >= 0 || (ddrift[1] <= ddrift[0] && ddrift[1] >= 0)
	if shortCondition && longCondition {
		if drift[1] > drift[0] {
			longCondition = false
		} else {
			shortCondition = false
		}
	}
	exitCondition := s.CheckStopLoss() || s.trailingCheck(pricef, "short") || s.trailingCheck(pricef, "long")

	if exitCondition {
		s.positionLock.Unlock()
		if err := s.GeneralOrderExecutor.GracefulCancel(ctx); err != nil {
			log.WithError(err).Errorf("cannot cancel orders")
			return
		}
		_ = s.ClosePosition(ctx, fixedpoint.One)
		if shortCondition || longCondition {
			s.positionLock.Lock()
		} else {
			return
		}
	}

	if longCondition {
		if err := s.GeneralOrderExecutor.GracefulCancel(ctx); err != nil {
			log.WithError(err).Errorf("cannot cancel orders")
			s.positionLock.Unlock()
			return
		}
		source = source.Sub(fixedpoint.NewFromFloat(s.stdevLow.Last() * s.HighLowVarianceMultiplier))
		if source.Compare(price) > 0 {
			source = price
		}
		/*source = fixedpoint.NewFromFloat(s.ma.Last() - s.stdevLow.Last()*s.HighLowVarianceMultiplier)
		if source.Compare(price) > 0 {
			source = price
		}
		sourcef = source.Float64()*/
		log.Infof("source in long %v %v %f", source, price, s.stdevLow.Last())

		s.positionLock.Unlock()
		opt := s.OpenPositionOptions
		opt.Long = true
		opt.Price = source
		opt.Tags = []string{"long"}
		createdOrders, err := s.GeneralOrderExecutor.OpenPosition(ctx, opt)
		if err != nil {
			errs := filterErrors(multierr.Errors(err))
			if len(errs) > 0 {
				log.Errorf("%v", errs)
				log.WithError(err).Errorf("cannot place buy order")
			}
			return
		}
		log.Infof("orders %v", createdOrders)
		if createdOrders != nil {
			s.orderPendingCounter[createdOrders[0].OrderID] = s.minutesCounter
		}
		return
	}
	if shortCondition {
		if err := s.GeneralOrderExecutor.GracefulCancel(ctx); err != nil {
			log.WithError(err).Errorf("cannot cancel orders")
			s.positionLock.Unlock()
			return
		}

		source = source.Add(fixedpoint.NewFromFloat(s.stdevHigh.Last() * s.HighLowVarianceMultiplier))
		if source.Compare(price) < 0 {
			source = price
		}
		/*source = fixedpoint.NewFromFloat(s.ma.Last() + s.stdevHigh.Last()*s.HighLowVarianceMultiplier)
		if source.Compare(price) < 0 {
			source = price
		}
		sourcef = source.Float64()*/

		log.Infof("source in short: %v", source)

		s.positionLock.Unlock()
		opt := s.OpenPositionOptions
		opt.Short = true
		opt.Price = source
		opt.Tags = []string{"short"}
		createdOrders, err := s.GeneralOrderExecutor.OpenPosition(ctx, opt)
		if err != nil {
			errs := filterErrors(multierr.Errors(err))
			if len(errs) > 0 {
				log.WithError(err).Errorf("cannot place sell order")
			}
			return
		}
		log.Infof("orders %v", createdOrders)
		if createdOrders != nil {
			s.orderPendingCounter[createdOrders[0].OrderID] = s.minutesCounter
		}
		return
	}
	s.positionLock.Unlock()
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.Leverage == fixedpoint.Zero {
		s.Leverage = fixedpoint.One
	}
	instanceID := s.InstanceID()
	// Will be set by persistence if there's any from DB
	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
		s.p = types.NewPositionFromMarket(s.Market)
	} else {
		s.p = types.NewPositionFromMarket(s.Market)
		s.p.Base = s.Position.Base
		s.p.Quote = s.Position.Quote
		s.p.AverageCost = s.Position.AverageCost
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
		_ = s.GeneralOrderExecutor.GracefulCancel(ctx)
	})

	s.OnEmergencyStop(func() {
		_ = s.GeneralOrderExecutor.GracefulCancel(ctx)
		_ = s.ClosePosition(ctx, fixedpoint.One)
	})

	s.GeneralOrderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.GeneralOrderExecutor.BindEnvironment(s.Environment)
	s.GeneralOrderExecutor.BindProfitStats(s.ProfitStats)
	s.GeneralOrderExecutor.BindTradeStats(s.TradeStats)
	s.GeneralOrderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})
	s.GeneralOrderExecutor.Bind()

	s.orderPendingCounter = make(map[uint64]int)
	s.minutesCounter = 0

	// Exit methods from config
	for _, method := range s.ExitMethods {
		method.Bind(session, s.GeneralOrderExecutor)
	}

	profit := floats.Slice{1., 1.}
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
	s.GeneralOrderExecutor.TradeCollector().OnTrade(func(trade types.Trade, _profit, _netProfit fixedpoint.Value) {
		s.p.AddTrade(trade)
		order, ok := s.GeneralOrderExecutor.TradeCollector().OrderStore().Get(trade.OrderID)
		if !ok {
			panic(fmt.Sprintf("cannot find order: %v", trade))
		}
		tag := order.Tag

		price := trade.Price.Float64()

		if s.buyPrice > 0 {
			profit.Update(modify(price / s.buyPrice))
			cumProfit.Update(s.CalcAssetValue(trade.Price).Float64())
		} else if s.sellPrice > 0 {
			profit.Update(modify(s.sellPrice / price))
			cumProfit.Update(s.CalcAssetValue(trade.Price).Float64())
		}
		s.positionLock.Lock()
		defer s.positionLock.Unlock()
		if s.p.IsDust(trade.Price) {
			s.buyPrice = 0
			s.sellPrice = 0
			s.highestPrice = 0
			s.lowestPrice = 0
		} else if s.p.IsLong() {
			s.buyPrice = s.p.ApproximateAverageCost.Float64() // trade.Price.Float64()
			s.sellPrice = 0
			s.highestPrice = math.Max(s.buyPrice, s.highestPrice)
			s.lowestPrice = s.buyPrice
		} else if s.p.IsShort() {
			s.sellPrice = s.p.ApproximateAverageCost.Float64() // trade.Price.Float64()
			s.buyPrice = 0
			s.highestPrice = s.sellPrice
			if s.lowestPrice == 0 {
				s.lowestPrice = s.sellPrice
			} else {
				s.lowestPrice = math.Min(s.lowestPrice, s.sellPrice)
			}
		}
		bbgo.Notify("tag: %s, sp: %.4f bp: %.4f hp: %.4f lp: %.4f, trade: %s, pos: %s", tag, s.sellPrice, s.buyPrice, s.highestPrice, s.lowestPrice, trade.String(), s.p.String())
	})

	s.frameKLine = &types.KLine{}
	s.kline1m = &types.KLine{}
	s.priceLines = types.NewQueue(300)

	s.initTickerFunctions(ctx)
	startTime := s.Environment.StartTime()
	s.TradeStats.SetIntervalProfitCollector(types.NewIntervalProfitCollector(types.Interval1d, startTime))
	s.TradeStats.SetIntervalProfitCollector(types.NewIntervalProfitCollector(types.Interval1w, startTime))

	// default value: use 1m kline
	if !s.NoTrailingStopLoss && s.IsBackTesting() || s.TrailingStopLossType == "" {
		s.TrailingStopLossType = "kline"
	}

	bbgo.RegisterCommand("/draw", "Draw Indicators", func(reply interact.Reply) {
		canvas := s.DrawIndicators(s.frameKLine.StartTime)
		var buffer bytes.Buffer
		if err := canvas.Render(chart.PNG, &buffer); err != nil {
			log.WithError(err).Errorf("cannot render indicators in drift")
			reply.Message(fmt.Sprintf("[error] cannot render indicators in drift: %v", err))
			return
		}
		bbgo.SendPhoto(&buffer)
	})

	bbgo.RegisterCommand("/pnl", "Draw PNL(%) per trade", func(reply interact.Reply) {
		canvas := s.DrawPNL(&profit)
		var buffer bytes.Buffer
		if err := canvas.Render(chart.PNG, &buffer); err != nil {
			log.WithError(err).Errorf("cannot render pnl in drift")
			reply.Message(fmt.Sprintf("[error] cannot render pnl in drift: %v", err))
			return
		}
		bbgo.SendPhoto(&buffer)
	})

	bbgo.RegisterCommand("/cumpnl", "Draw Cummulative PNL(Quote)", func(reply interact.Reply) {
		canvas := s.DrawCumPNL(&cumProfit)
		var buffer bytes.Buffer
		if err := canvas.Render(chart.PNG, &buffer); err != nil {
			log.WithError(err).Errorf("cannot render cumpnl in drift")
			reply.Message(fmt.Sprintf("[error] canot render cumpnl in drift: %v", err))
			return
		}
		bbgo.SendPhoto(&buffer)
	})

	bbgo.RegisterCommand("/config", "Show latest config", func(reply interact.Reply) {
		var buffer bytes.Buffer
		s.Print(&buffer, false)
		reply.Message(buffer.String())
	})

	bbgo.RegisterCommand("/pos", "Show internal position", func(reply interact.Reply) {
		reply.Message(s.p.String())
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

	// event trigger order: s.Interval => Interval1m
	store, ok := session.SerialMarketDataStore(s.Symbol, []types.Interval{s.Interval, types.Interval1m})
	if !ok {
		panic("cannot get 1m history")
	}
	if err := s.initIndicators(store); err != nil {
		log.WithError(err).Errorf("initIndicator failed")
		return nil
	}
	store.OnKLineClosed(func(kline types.KLine) {
		s.minutesCounter = int(kline.StartTime.Time().Add(kline.Interval.Duration()).Sub(s.startTime).Minutes())
		if kline.Interval == s.Interval {
			s.klineHandler(ctx, kline)
		} else if kline.Interval == types.Interval1m {
			s.klineHandler1m(ctx, kline)
		}
	})

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {

		var buffer bytes.Buffer

		s.Print(&buffer, true, true)

		fmt.Fprintln(&buffer, "--- NonProfitable Dates ---")
		for _, daypnl := range s.TradeStats.IntervalProfits[types.Interval1d].GetNonProfitableIntervals() {
			fmt.Fprintf(&buffer, "%s\n", daypnl)
		}
		fmt.Fprintln(&buffer, s.TradeStats.BriefString())

		os.Stdout.Write(buffer.Bytes())

		if s.GenerateGraph {
			s.Draw(s.frameKLine.StartTime, &profit, &cumProfit)
		}
		wg.Done()
	})
	return nil
}
