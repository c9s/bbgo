package drift

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/wcharczuk/go-chart/v2"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/interact"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

const ID = "drift"

const DDriftFilterNeg = -0.7
const DDriftFilterPos = 0.7
const DriftFilterNeg = -1.85
const DriftFilterPos = 1.85

var log = logrus.WithField("strategy", ID)
var Four fixedpoint.Value = fixedpoint.NewFromInt(4)
var Three fixedpoint.Value = fixedpoint.NewFromInt(3)
var Two fixedpoint.Value = fixedpoint.NewFromInt(2)
var Delta fixedpoint.Value = fixedpoint.NewFromFloat(0.01)
var Fee = 0.0008 // taker fee % * 2, for upper bound

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type SourceFunc func(*types.KLine) fixedpoint.Value

type Strategy struct {
	Symbol string `json:"symbol"`

	bbgo.StrategyController
	types.Market
	types.IntervalWindow

	*bbgo.Environment
	*types.Position    `persistence:"position"`
	*types.ProfitStats `persistence:"profit_stats"`
	*types.TradeStats  `persistence:"trade_stats"`

	p *types.Position

	trendLine           types.UpdatableSeriesExtend
	ma                  types.UpdatableSeriesExtend
	stdevHigh           *indicator.StdDev
	stdevLow            *indicator.StdDev
	drift               *DriftMA
	drift1m             *DriftMA
	atr                 *indicator.ATR
	midPrice            fixedpoint.Value
	lock                sync.RWMutex `ignore:"true"`
	positionLock        sync.RWMutex `ignore:"true"`
	minutesCounter      int
	orderPendingCounter map[uint64]int
	frameKLine          *types.KLine
	kline1m             *types.KLine

	beta float64

	// This stores the maximum TP coefficient of ATR multiplier of each entry point
	takeProfitFactor types.UpdatableSeriesExtend

	Source                    string           `json:"source,omitempty"`
	TakeProfitFactor          float64          `json:"takeProfitFactor"`
	ProfitFactorWindow        int              `json:"profitFactorWindow"`
	StopLoss                  fixedpoint.Value `json:"stoploss"`
	CanvasPath                string           `json:"canvasPath"`
	PredictOffset             int              `json:"predictOffset"`
	HighLowVarianceMultiplier float64          `json:"hlVarianceMultiplier"`
	NoTrailingStopLoss        bool             `json:"noTrailingStopLoss"`
	TrailingStopLossType      string           `json:"trailingStopLossType"` // trailing stop sources. Possible options are `kline` for 1m kline and `realtime` from order updates
	HLRangeWindow             int              `json:"hlRangeWindow"`
	SmootherWindow            int              `json:"smootherWindow"`
	FisherTransformWindow     int              `json:"fisherTransformWindow"`
	ATRWindow                 int              `json:"atrWindow"`
	PendingMinutes            int              `json:"pendingMinutes"`  // if order not be traded for pendingMinutes of time, cancel it.
	NoRebalance               bool             `json:"noRebalance"`     // disable rebalance
	TrendWindow               int              `json:"trendWindow"`     // trendLine is used for rebalancing the position. When trendLine goes up, hold base, otherwise hold quote
	RebalanceFilter           float64          `json:"rebalanceFilter"` // beta filter on the Linear Regression of trendLine
	TrailingCallbackRate      []float64        `json:"trailingCallbackRate"`
	TrailingActivationRatio   []float64        `json:"trailingActivationRatio"`

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
	getSource    SourceFunc
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s:%v", ID, "" /*s.Symbol*/, bbgo.IsBackTesting)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{
		Interval: s.Interval,
	})
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

func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	order := s.p.NewMarketCloseOrder(percentage)
	if order == nil {
		return nil
	}
	order.Tag = "close"
	order.TimeInForce = ""
	balances := s.GeneralOrderExecutor.Session().GetAccount().Balances()
	baseBalance := balances[s.Market.BaseCurrency].Available
	price := s.getLastPrice()
	if order.Side == types.SideTypeBuy {
		quoteAmount := balances[s.Market.QuoteCurrency].Available.Div(price)
		if order.Quantity.Compare(quoteAmount) > 0 {
			order.Quantity = quoteAmount
		}
	} else if order.Side == types.SideTypeSell && order.Quantity.Compare(baseBalance) > 0 {
		order.Quantity = baseBalance
	}
	for {
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
}

func (s *Strategy) SourceFuncGenerator() SourceFunc {
	switch strings.ToLower(s.Source) {
	case "close":
		return func(kline *types.KLine) fixedpoint.Value { return kline.Close }
	case "high":
		return func(kline *types.KLine) fixedpoint.Value { return kline.High }
	case "low":
		return func(kline *types.KLine) fixedpoint.Value { return kline.Low }
	case "hl2":
		return func(kline *types.KLine) fixedpoint.Value {
			return kline.High.Add(kline.Low).Div(Two)
		}
	case "hlc3":
		return func(kline *types.KLine) fixedpoint.Value {
			return kline.High.Add(kline.Low).Add(kline.Close).Div(Three)
		}
	case "ohlc4":
		return func(kline *types.KLine) fixedpoint.Value {
			return kline.Open.Add(kline.High).Add(kline.Low).Add(kline.Close).Div(Four)
		}
	case "open":
		return func(kline *types.KLine) fixedpoint.Value { return kline.Open }
	case "":
		log.Infof("source not set, use hl2 by default")
		return func(kline *types.KLine) fixedpoint.Value {
			return kline.High.Add(kline.Low).Div(Two)
		}
	default:
		panic(fmt.Sprintf("Unable to parse: %s", s.Source))
	}
}

func (s *Strategy) initIndicators(priceLines *types.Queue) error {
	s.ma = &indicator.SMA{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.HLRangeWindow}}
	s.stdevHigh = &indicator.StdDev{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.HLRangeWindow}}
	s.stdevLow = &indicator.StdDev{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.HLRangeWindow}}
	s.drift = &DriftMA{
		drift: &indicator.Drift{
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
	s.drift1m = &DriftMA{
		drift: &indicator.Drift{
			MA:             &indicator.SMA{IntervalWindow: types.IntervalWindow{Interval: types.Interval1m, Window: 2}},
			IntervalWindow: types.IntervalWindow{Interval: types.Interval1m, Window: 2},
		},
		ma1: &indicator.EWMA{
			IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: 24},
		},
		ma2: &indicator.FisherTransform{
			IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.FisherTransformWindow * 15},
		},
	}
	s.drift1m.SeriesBase.Series = s.drift1m
	s.atr = &indicator.ATR{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.ATRWindow}}
	s.takeProfitFactor = &indicator.SMA{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.ProfitFactorWindow}}
	s.trendLine = &indicator.EWMA{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.TrendWindow}}

	for i := 0; i <= s.ProfitFactorWindow; i++ {
		s.takeProfitFactor.Update(s.TakeProfitFactor)
	}
	store, _ := s.Session.MarketDataStore(s.Symbol)
	klines, ok := store.KLinesOfInterval(s.Interval)
	if !ok {
		return errors.New("klines not exists")
	}

	for _, kline := range *klines {
		source := s.getSource(&kline).Float64()
		high := kline.High.Float64()
		low := kline.Low.Float64()
		s.ma.Update(source)
		s.stdevHigh.Update(high - s.ma.Last())
		s.stdevLow.Update(s.ma.Last() - low)
		s.drift.Update(source)
		s.trendLine.Update(source)
		s.atr.PushK(kline)
		priceLines.Update(source)
	}
	if s.frameKLine != nil && klines != nil {
		s.frameKLine.Set(&(*klines)[len(*klines)-1])
	}
	klines, ok = store.KLinesOfInterval(types.Interval1m)
	if !ok {
		return errors.New("klines not exists")
	}
	for _, kline := range *klines {
		source := s.getSource(&kline).Float64()
		s.drift1m.Update(source)
	}
	if s.kline1m != nil && klines != nil {
		s.kline1m.Set(&(*klines)[len(*klines)-1])
	}
	return nil
}

func (s *Strategy) smartCancel(ctx context.Context, pricef, atr, takeProfitFactor float64) (int, error) {
	nonTraded := s.GeneralOrderExecutor.ActiveMakerOrders().Orders()
	if len(nonTraded) > 0 {
		if len(nonTraded) > 1 {
			log.Errorf("should only have one order to cancel, got %d", len(nonTraded))
		}
		toCancel := false

		drift := s.drift1m.Array(2)
		for _, order := range nonTraded {
			log.Warnf("%v", order)
			if s.minutesCounter-s.orderPendingCounter[order.OrderID] > s.PendingMinutes {
				if order.Side == types.SideTypeBuy && drift[1] < drift[0] {
					continue
				} else if order.Side == types.SideTypeSell && drift[1] > drift[0] {
					continue
				}
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
			if err == nil {
				for _, order := range nonTraded {
					delete(s.orderPendingCounter, order.OrderID)
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
	for i := len(s.TrailingCallbackRate) - 1; i >= 0; i-- {
		trailingCallbackRate := s.TrailingCallbackRate[i]
		trailingActivationRatio := s.TrailingActivationRatio[i]
		if isShort {
			if (s.sellPrice-s.lowestPrice)/s.lowestPrice > trailingActivationRatio {
				return (price-s.lowestPrice)/s.lowestPrice > trailingCallbackRate
			}
		} else {
			if (s.highestPrice-s.buyPrice)/s.buyPrice > trailingActivationRatio {
				return (s.highestPrice-price)/price > trailingCallbackRate
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

			var pricef, atr float64
			if util.TryLock(&s.lock) {
				if !bestAsk.IsZero() && !bestBid.IsZero() {
					s.midPrice = bestAsk.Add(bestBid).Div(Two)
				} else if !bestAsk.IsZero() {
					s.midPrice = bestAsk
				} else {
					s.midPrice = bestBid
				}
				pricef = s.midPrice.Float64()
			} else {
				return
			}

			s.lock.Unlock()
			// for trailing stoploss during the realtime
			if s.NoTrailingStopLoss || s.TrailingStopLossType == "kline" {
				return
			}

			stoploss := s.StopLoss.Float64()
			atr = s.atr.Last()
			takeProfitFactor := s.takeProfitFactor.Predict(2)
			numPending := 0
			var err error
			if numPending, err = s.smartCancel(ctx, pricef, atr, takeProfitFactor); err != nil {
				log.WithError(err).Errorf("cannot cancel orders")
				return
			}
			if numPending > 0 {
				return
			}

			s.positionLock.Lock()

			if s.highestPrice > 0 && s.highestPrice < pricef {
				s.highestPrice = pricef
			}
			if s.lowestPrice > 0 && s.lowestPrice > pricef {
				s.lowestPrice = pricef
			}

			exitShortCondition := s.sellPrice > 0 && (s.sellPrice*(1.+stoploss) <= pricef || s.sellPrice-atr*takeProfitFactor >= pricef ||
				s.trailingCheck(pricef, "short"))
			exitLongCondition := s.buyPrice > 0 && (s.buyPrice*(1.-stoploss) >= pricef || s.buyPrice+atr*takeProfitFactor <= pricef ||
				s.trailingCheck(pricef, "long"))
			if exitShortCondition || exitLongCondition {
				if exitLongCondition && s.highestPrice > s.buyPrice {
					s.takeProfitFactor.Update((s.highestPrice - s.buyPrice) / atr * 1.5)
				}
				if exitShortCondition && s.sellPrice > s.lowestPrice {
					s.takeProfitFactor.Update((s.sellPrice - s.lowestPrice) / atr * 1.5)
				}
				log.Infof("Close position by orderbook changes")
				s.positionLock.Unlock()
				_ = s.ClosePosition(ctx, fixedpoint.One)
			} else {
				s.positionLock.Unlock()
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

func (s *Strategy) DrawIndicators(time types.Time, priceLine types.SeriesExtend, zeroPoints types.Series) *types.Canvas {
	canvas := types.NewCanvas(s.InstanceID(), s.Interval)
	Length := priceLine.Length()
	if Length > 300 {
		Length = 300
	}
	log.Infof("draw indicators with %d data", Length)
	mean := priceLine.Mean(Length)
	highestPrice := priceLine.Minus(mean).Abs().Highest(Length)
	highestDrift := s.drift.Abs().Highest(Length)
	hi := s.drift.drift.Abs().Highest(Length)
	ratio := highestPrice / highestDrift
	canvas.Plot("upband", s.ma.Add(s.stdevHigh), time, Length)
	canvas.Plot("ma", s.ma, time, Length)
	canvas.Plot("downband", s.ma.Minus(s.stdevLow), time, Length)
	canvas.Plot("drift", s.drift.Mul(ratio).Add(mean), time, Length)
	canvas.Plot("driftOrig", s.drift.drift.Mul(highestPrice/hi).Add(mean), time, Length)
	canvas.Plot("zero", types.NumberSeries(mean), time, Length)
	canvas.Plot("price", priceLine, time, Length)
	canvas.Plot("zeroPoint", zeroPoints, time, Length)
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

func (s *Strategy) Draw(time types.Time, priceLine types.SeriesExtend, profit types.Series, cumProfit types.Series, zeroPoints types.Series) {
	canvas := s.DrawIndicators(time, priceLine, zeroPoints)
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

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
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
	startTime := s.Environment.StartTime()
	s.TradeStats.SetIntervalProfitCollector(types.NewIntervalProfitCollector(types.Interval1d, startTime))
	s.TradeStats.SetIntervalProfitCollector(types.NewIntervalProfitCollector(types.Interval1w, startTime))

	// StrategyController
	s.Status = types.StrategyStatusRunning
	// Get source function from config input
	s.getSource = s.SourceFuncGenerator()

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
		bbgo.Sync(s)
	})
	s.GeneralOrderExecutor.Bind()

	s.orderPendingCounter = make(map[uint64]int)
	s.minutesCounter = 0

	// Exit methods from config
	for _, method := range s.ExitMethods {
		method.Bind(session, s.GeneralOrderExecutor)
	}
	profit := types.Float64Slice{}
	cumProfit := types.Float64Slice{}
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
		balances := s.Session.GetAccount().Balances()

		if s.buyPrice > 0 {
			profit.Update(modify(price / s.buyPrice))
			asset := balances[s.Market.BaseCurrency].Total().Mul(trade.Price).Add(balances[s.Market.QuoteCurrency].Total())
			cumProfit.Update(asset.Float64())
		} else if s.sellPrice > 0 {
			profit.Update(modify(s.sellPrice / price))
			asset := balances[s.Market.BaseCurrency].Total().Mul(trade.Price).Add(balances[s.Market.QuoteCurrency].Total())
			cumProfit.Update(asset.Float64())
		}
		s.positionLock.Lock()
		defer s.positionLock.Unlock()
		if tag == "close" {
			if s.p.IsDust(trade.Price) {
				s.buyPrice = 0
				s.sellPrice = 0
				s.highestPrice = 0
				s.lowestPrice = 0
			} else if s.p.IsLong() {

				s.buyPrice = trade.Price.Float64()
				s.sellPrice = 0
				s.highestPrice = s.buyPrice
				s.lowestPrice = 0
			} else {
				s.sellPrice = trade.Price.Float64()
				s.buyPrice = 0
				s.highestPrice = 0
				s.lowestPrice = s.sellPrice
			}
		} else if tag == "long" {
			if s.p.IsDust(trade.Price) {
				s.buyPrice = 0
				s.sellPrice = 0
				s.highestPrice = 0
				s.lowestPrice = 0
			} else if s.p.IsLong() {
				s.buyPrice = trade.Price.Float64()
				s.sellPrice = 0
				s.highestPrice = s.buyPrice
				s.lowestPrice = 0
			}
		} else if tag == "short" {
			if s.p.IsDust(trade.Price) {
				s.sellPrice = 0
				s.buyPrice = 0
				s.highestPrice = 0
				s.lowestPrice = 0
			} else if s.p.IsShort() {
				s.sellPrice = trade.Price.Float64()
				s.buyPrice = 0
				s.highestPrice = 0
				s.lowestPrice = s.sellPrice
			}
		} else {
			panic("tag unknown")
		}
		bbgo.Notify("tag: %s, sp: %.4f bp: %.4f hp: %.4f lp: %.4f, trade: %s, pos: %s", tag, s.sellPrice, s.buyPrice, s.highestPrice, s.lowestPrice, trade.String(), s.p.String())
	})

	s.frameKLine = &types.KLine{}
	s.kline1m = &types.KLine{}
	priceLine := types.NewQueue(300)
	if err := s.initIndicators(priceLine); err != nil {
		log.WithError(err).Errorf("initIndicator failed")
		return nil
	}
	s.initTickerFunctions(ctx)

	zeroPoints := types.NewQueue(300)
	stoploss := s.StopLoss.Float64()
	// default value: use 1m kline
	if !s.NoTrailingStopLoss && s.IsBackTesting() || s.TrailingStopLossType == "" {
		s.TrailingStopLossType = "kline"
	}

	bbgo.RegisterCommand("/draw", "Draw Indicators", func(reply interact.Reply) {
		canvas := s.DrawIndicators(s.frameKLine.StartTime, priceLine, zeroPoints)
		var buffer bytes.Buffer
		if err := canvas.Render(chart.PNG, &buffer); err != nil {
			log.WithError(err).Errorf("cannot render indicators in drift")
			reply.Message(fmt.Sprintf("[error] cannot render indicators in drift: %v", err))
			return
		}
		bbgo.SendPhoto(&buffer)
	})

	bbgo.RegisterCommand("/pnl", "Draw PNL per trade", func(reply interact.Reply) {
		canvas := s.DrawPNL(&profit)
		var buffer bytes.Buffer
		if err := canvas.Render(chart.PNG, &buffer); err != nil {
			log.WithError(err).Errorf("cannot render pnl in drift")
			reply.Message(fmt.Sprintf("[error] cannot render pnl in drift: %v", err))
			return
		}
		bbgo.SendPhoto(&buffer)
	})

	bbgo.RegisterCommand("/cumpnl", "Draw Cummulative PNL", func(reply interact.Reply) {
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
			s.ParamDump(&buffer)
		} else {
			s.ParamDump(&buffer, l)
		}
		reply.Message(buffer.String())
	})

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != s.Symbol {
			return
		}
		var driftPred, atr float64
		var drift []float64

		if !kline.Closed {
			return
		}
		if kline.Interval == types.Interval1m {
			s.kline1m.Set(&kline)
			s.drift1m.Update(s.getSource(&kline).Float64())
			s.minutesCounter += 1
			if s.Status != types.StrategyStatusRunning {
				return
			}
			if s.NoTrailingStopLoss || s.TrailingStopLossType == "realtime" {
				return
			}
			// for doing the trailing stoploss during backtesting
			atr = s.atr.Last()
			price := s.getLastPrice()
			pricef := price.Float64()

			takeProfitFactor := s.takeProfitFactor.Predict(2)
			var err error
			numPending := 0
			if numPending, err = s.smartCancel(ctx, pricef, atr, takeProfitFactor); err != nil {
				log.WithError(err).Errorf("cannot cancel orders")
				return
			}
			if numPending > 0 {
				log.Infof("pending orders: %d, exit", numPending)
				return
			}

			lowf := math.Min(kline.Low.Float64(), pricef)
			highf := math.Max(kline.High.Float64(), pricef)
			s.positionLock.Lock()
			if s.lowestPrice > 0 && lowf < s.lowestPrice {
				s.lowestPrice = lowf
			}
			if s.highestPrice > 0 && highf > s.highestPrice {
				s.highestPrice = highf
			}
			exitShortCondition := s.sellPrice > 0 && (s.sellPrice*(1.+stoploss) <= highf || s.sellPrice-atr*takeProfitFactor >= lowf ||
				s.trailingCheck(highf, "short"))
			exitLongCondition := s.buyPrice > 0 && (s.buyPrice*(1.-stoploss) >= lowf || s.buyPrice+atr*takeProfitFactor <= highf ||
				s.trailingCheck(lowf, "long"))
			if exitShortCondition || exitLongCondition {
				if exitLongCondition && s.highestPrice > s.buyPrice {
					s.takeProfitFactor.Update((s.highestPrice - s.buyPrice) / atr * 1.5)
				}
				if exitShortCondition && s.sellPrice > s.lowestPrice {
					s.takeProfitFactor.Update((s.sellPrice - s.lowestPrice) / atr * 1.5)
				}
				s.positionLock.Unlock()
				_ = s.ClosePosition(ctx, fixedpoint.One)
			} else {
				s.positionLock.Unlock()
			}
			return
		}
		if kline.Interval != s.Interval {
			return
		}
		s.frameKLine.Set(&kline)

		source := s.getSource(s.frameKLine)
		sourcef := source.Float64()
		priceLine.Update(sourcef)
		s.ma.Update(sourcef)
		s.trendLine.Update(sourcef)
		s.drift.Update(sourcef)

		zeroPoint := s.drift.ZeroPoint()
		zeroPoints.Update(zeroPoint)
		s.atr.PushK(kline)
		drift = s.drift.Array(2)
		ddrift := s.drift.drift.Array(2)
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

		if s.Status != types.StrategyStatusRunning {
			return
		}

		s.positionLock.Lock()
		log.Errorf("highdiff: %3.2f ma: %.2f, close: %8v, high: %8v, low: %8v, time: %v", s.stdevHigh.Last(), s.ma.Last(), kline.Close, kline.High, kline.Low, kline.StartTime)
		if s.lowestPrice > 0 && lowf < s.lowestPrice {
			s.lowestPrice = lowf
		}
		if s.highestPrice > 0 && highf > s.highestPrice {
			s.highestPrice = highf
		}
		takeProfitFactor := s.takeProfitFactor.Predict(2)

		if !s.NoRebalance {
			s.Rebalance(ctx)
		}

		balances := s.GeneralOrderExecutor.Session().GetAccount().Balances()
		bbgo.Notify("source: %.4f, price: %.4f, driftPred: %.4f, ddriftPred: %.4f, drift[1]: %.4f, ddrift[1]: %.4f, atr: %.4f, takeProfitFact: %.4f, lowf %.4f, highf: %.4f lowest: %.4f highest: %.4f sp %.4f bp %.4f",
			sourcef, pricef, driftPred, ddriftPred, drift[1], ddrift[1], atr, takeProfitFactor, lowf, highf, s.lowestPrice, s.highestPrice, s.sellPrice, s.buyPrice)
		// Notify will parse args to strings and process separately
		bbgo.Notify("balances: [Base] %s(%vU) [Quote] %s",
			balances[s.Market.BaseCurrency].String(),
			balances[s.Market.BaseCurrency].Total().Mul(price),
			balances[s.Market.QuoteCurrency].String())

		shortCondition := (drift[1] >= DriftFilterNeg || ddrift[1] >= 0) && (driftPred <= DDriftFilterNeg || ddriftPred <= 0) || drift[1] < 0 && drift[0] < 0
		longCondition := (drift[1] <= DriftFilterPos || ddrift[1] <= 0) && (driftPred >= DDriftFilterPos || ddriftPred >= 0) || drift[1] > 0 && drift[0] > 0
		if shortCondition && longCondition {
			if drift[1] > drift[0] {
				longCondition = false
			} else {
				shortCondition = false
			}
		}
		exitShortCondition := s.sellPrice > 0 && ( //!shortCondition && !longCondition ||
		s.sellPrice*(1.+stoploss) <= highf ||
			s.trailingCheck(pricef, "short") ||
			s.sellPrice-atr*takeProfitFactor >= s.lowestPrice)
		exitLongCondition := s.buyPrice > 0 && ( //!longCondition && !shortCondition ||
		s.buyPrice*(1.-stoploss) >= lowf ||
			s.trailingCheck(pricef, "long") ||
			s.buyPrice+atr*takeProfitFactor <= s.highestPrice)

		if exitShortCondition || exitLongCondition {
			if err := s.GeneralOrderExecutor.GracefulCancel(ctx); err != nil {
				log.WithError(err).Errorf("cannot cancel orders")
				s.positionLock.Unlock()
				return
			}
			if exitShortCondition && s.sellPrice > s.lowestPrice {
				s.takeProfitFactor.Update((s.sellPrice - s.lowestPrice) / atr * 1.5)
			}
			if exitLongCondition && s.buyPrice < s.highestPrice {
				s.takeProfitFactor.Update((s.highestPrice - s.buyPrice) / atr * 1.5)
			}
			if s.takeProfitFactor.Last() == 0 {
				log.Errorf("exit %f %f %v", s.highestPrice, s.lowestPrice, s.takeProfitFactor.Array(10))
			}
			s.positionLock.Unlock()
			_ = s.ClosePosition(ctx, fixedpoint.One)
			return
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
			sourcef = source.Float64()

			quoteBalance, ok := s.Session.GetAccount().Balance(s.Market.QuoteCurrency)
			if !ok {
				log.Errorf("unable to get quoteCurrency")
				s.positionLock.Unlock()
				return
			}
			if s.Market.IsDustQuantity(
				quoteBalance.Available.Div(source), source) {
				s.positionLock.Unlock()
				return
			}
			if s.sellPrice > s.lowestPrice && s.lowestPrice > 0 {
				s.takeProfitFactor.Update((s.sellPrice - s.lowestPrice) / atr * 1.5)
			}
			s.positionLock.Unlock()
			quantity := quoteBalance.Available.Div(source)
			createdOrders, err := s.GeneralOrderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:   s.Symbol,
				Side:     types.SideTypeBuy,
				Type:     types.OrderTypeLimit,
				Price:    source,
				Quantity: quantity,
				Tag:      "long",
			})
			if err != nil {
				log.WithError(err).Errorf("cannot place buy order")
				return
			}
			s.orderPendingCounter[createdOrders[0].OrderID] = s.minutesCounter
			return
		}
		if shortCondition {
			if err := s.GeneralOrderExecutor.GracefulCancel(ctx); err != nil {
				log.WithError(err).Errorf("cannot cancel orders")
				s.positionLock.Unlock()
				return
			}
			baseBalance, ok := s.Session.GetAccount().Balance(s.Market.BaseCurrency)
			if !ok {
				log.Errorf("unable to get baseBalance")
				s.positionLock.Unlock()
				return
			}
			source = source.Add(fixedpoint.NewFromFloat(s.stdevHigh.Last() * s.HighLowVarianceMultiplier))
			if source.Compare(price) < 0 {
				source = price
			}
			sourcef = source.Float64()

			if s.Market.IsDustQuantity(baseBalance.Available, source) {
				s.positionLock.Unlock()
				return
			}
			if s.buyPrice < s.highestPrice && s.buyPrice > 0 {
				s.takeProfitFactor.Update((s.highestPrice - s.buyPrice) / atr * 1.5)
			}
			s.positionLock.Unlock()
			// Cleanup pending StopOrders
			quantity := baseBalance.Available
			createdOrders, err := s.GeneralOrderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:   s.Symbol,
				Side:     types.SideTypeSell,
				Type:     types.OrderTypeLimit,
				Price:    source,
				Quantity: quantity,
				Tag:      "short",
			})
			if err != nil {
				log.WithError(err).Errorf("cannot place sell order")
				return
			}
			s.orderPendingCounter[createdOrders[0].OrderID] = s.minutesCounter
			return
		}
	})

	bbgo.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {

		var buffer bytes.Buffer

		s.Print(&buffer, true, true)
		fmt.Fprintln(&buffer, s.TradeStats.BriefString())

		os.Stdout.Write(buffer.Bytes())

		if s.GenerateGraph {
			s.Draw(s.frameKLine.StartTime, priceLine, &profit, &cumProfit, zeroPoints)
		}

		wg.Done()
	})
	return nil
}
