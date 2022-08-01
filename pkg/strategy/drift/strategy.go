package drift

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"github.com/wcharczuk/go-chart/v2"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

const ID = "drift"

var log = logrus.WithField("strategy", ID)
var Four fixedpoint.Value = fixedpoint.NewFromInt(4)
var Three fixedpoint.Value = fixedpoint.NewFromInt(3)
var Two fixedpoint.Value = fixedpoint.NewFromInt(2)
var Delta fixedpoint.Value = fixedpoint.NewFromFloat(0.01)
var Fee = fixedpoint.NewFromFloat(0.0008) // taker fee % * 2, for upper bound

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

	ma        types.UpdatableSeriesExtend
	stdevHigh *indicator.StdDev
	stdevLow  *indicator.StdDev
	drift     *DriftMA
	atr       *indicator.ATR
	midPrice  fixedpoint.Value
	lock      sync.RWMutex

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
	HLRangeWindow             int              `json:"hlRangeWindow"`
	SmootherWindow            int              `json:"smootherWindow"`
	FisherTransformWindow     int              `json:"fisherTransformWindow"`
	ATRWindow                 int              `json:"atrWindow"`
	OrderFeeProtection        bool             `json:"orderTakerFeeProtection"`

	buyPrice     float64
	sellPrice    float64
	highestPrice float64
	lowestPrice  float64

	buyOrderFeePriceFactor fixedpoint.Value

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

func (s *Strategy) Print(o *os.File) {
	f := bufio.NewWriter(o)
	defer f.Flush()
	b, _ := json.MarshalIndent(s.ExitMethods, "  ", "  ")
	hiyellow := color.New(color.FgHiYellow).FprintfFunc()
	hiyellow(f, "------ %s Settings ------\n", s.InstanceID())
	hiyellow(f, "generateGraph: %v\n", s.GenerateGraph)
	hiyellow(f, "canvasPath: %s\n", s.CanvasPath)
	hiyellow(f, "graphPNLPath: %s\n", s.GraphPNLPath)
	hiyellow(f, "graphCumPNLPath: %s\n", s.GraphCumPNLPath)
	hiyellow(f, "source: %s\n", s.Source)
	hiyellow(f, "stoploss: %v\n", s.StopLoss)
	hiyellow(f, "takeProfitFactor(last): %f, (init): %f\n", s.takeProfitFactor.Last(), s.TakeProfitFactor)
	hiyellow(f, "profitFactorWindow: %d\n", s.ProfitFactorWindow)
	hiyellow(f, "predictOffset: %d\n", s.PredictOffset)
	hiyellow(f, "exits:\n %s\n", string(b))
	hiyellow(f, "symbol: %s\n", s.Symbol)
	hiyellow(f, "interval: %s\n", s.Interval)
	hiyellow(f, "window: %d\n", s.Window)
	hiyellow(f, "noTrailingStopLoss: %v\n", s.NoTrailingStopLoss)
	hiyellow(f, "hlVarianceMutiplier: %f\n", s.HighLowVarianceMultiplier)
	hiyellow(f, "hlRangeWindow: %d\n", s.HLRangeWindow)
	hiyellow(f, "smootherWindow: %d\n", s.SmootherWindow)
	hiyellow(f, "fisherTransformWindow: %d\n", s.FisherTransformWindow)
	hiyellow(f, "atrWindow: %d\n", s.ATRWindow)
	hiyellow(f, "orderFeeProtection: %t\n", s.OrderFeeProtection)
	hiyellow(f, "\n")
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
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
	order := s.Position.NewMarketCloseOrder(percentage)
	if order == nil {
		return nil
	}
	order.Tag = "close"
	order.TimeInForce = ""
	balances := s.Session.GetAccount().Balances()
	baseBalance := balances[s.Market.BaseCurrency].Available
	price := s.getLastPrice()
	if order.Side == types.SideTypeBuy {
		quoteAmount := balances[s.Market.QuoteCurrency].Available.Div(price.Mul(s.buyOrderFeePriceFactor))
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

type DriftMA struct {
	types.SeriesBase
	ma1   types.UpdatableSeriesExtend
	drift *indicator.Drift
	ma2   types.UpdatableSeriesExtend
}

func (s *DriftMA) Update(value float64) {
	s.ma1.Update(value)
	s.drift.Update(s.ma1.Last())
	s.ma2.Update(s.drift.Last())
}

func (s *DriftMA) Last() float64 {
	return s.ma2.Last()
}

func (s *DriftMA) Index(i int) float64 {
	return s.ma2.Index(i)
}

func (s *DriftMA) Length() int {
	return s.ma2.Length()
}

func (s *DriftMA) ZeroPoint() float64 {
	return s.drift.ZeroPoint()
}

func (s *DriftMA) Clone() *DriftMA {
	out := DriftMA{
		ma1:   types.Clone(s.ma1),
		drift: s.drift.Clone(),
		ma2:   types.Clone(s.ma2),
	}
	out.SeriesBase.Series = &out
	return &out
}

func (s *DriftMA) TestUpdate(v float64) *DriftMA {
	out := s.Clone()
	out.Update(v)
	return out
}

func (s *Strategy) initIndicators() error {
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
	s.atr = &indicator.ATR{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.ATRWindow}}
	s.takeProfitFactor = &indicator.SMA{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.ProfitFactorWindow}}
	for i := 0; i < s.ProfitFactorWindow; i++ {
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
		s.atr.PushK(kline)
	}
	return nil
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

			var pricef, atr, avg float64
			var price fixedpoint.Value
			if util.TryLock(&s.lock) {
				if !bestAsk.IsZero() && !bestBid.IsZero() {
					s.midPrice = bestAsk.Add(bestBid).Div(Two)
				} else if !bestAsk.IsZero() {
					s.midPrice = bestAsk
				} else {
					s.midPrice = bestBid
				}
				price = s.midPrice
				pricef = s.midPrice.Float64()
			} else {
				return
			}
			if s.highestPrice > 0 && s.highestPrice < pricef {
				s.highestPrice = pricef
			}
			if s.lowestPrice > 0 && s.lowestPrice > pricef {
				s.lowestPrice = pricef
			}

			// for trailing stoploss during the realtime
			if s.NoTrailingStopLoss || s.GeneralOrderExecutor.ActiveMakerOrders().NumOfOrders() > 0 {
				s.lock.Unlock()
				return
			}
			atr = s.atr.Last()
			avg = s.buyPrice + s.sellPrice
			d := s.drift.TestUpdate(pricef)
			drift := d.Last()
			ddrift := d.drift.Last()
			takeProfitFactor := s.takeProfitFactor.Predict(2)
			exitShortCondition := ( /*avg+atr/2 <= pricef || avg*(1.+stoploss) <= pricef ||*/ (drift > 0 && ddrift > 0.6) || avg-atr*takeProfitFactor >= pricef ||
				((pricef-s.lowestPrice)/s.lowestPrice > 0.003 && (avg-s.lowestPrice)/s.lowestPrice > 0.015)) &&
				(s.Position.IsShort() && !s.Position.IsDust(price))
			exitLongCondition := ( /*avg-atr/2 >= pricef || avg*(1.-stoploss) >= pricef ||*/ (drift < 0 && ddrift < -0.6) || avg+atr*takeProfitFactor <= pricef ||
				((s.highestPrice-pricef)/pricef > 0.003 && (s.highestPrice-avg)/avg > 0.015)) &&
				(!s.Position.IsLong() && !s.Position.IsDust(price))
			if exitShortCondition || exitLongCondition {
				if exitLongCondition && s.highestPrice > avg {
					s.takeProfitFactor.Update((s.highestPrice - avg) / atr * 4)
				} else if exitShortCondition && avg > s.lowestPrice {
					s.takeProfitFactor.Update((avg - s.lowestPrice) / atr * 4)
				}
				_ = s.ClosePosition(ctx, fixedpoint.One)
			}
			s.lock.Unlock()

		})
		s.getLastPrice = func() (lastPrice fixedpoint.Value) {
			var ok bool
			s.lock.RLock()
			if s.midPrice.IsZero() {
				lastPrice, ok = s.Session.LastPrice(s.Symbol)
				if !ok {
					log.Error("cannot get lastprice")
					return lastPrice
				}
			} else {
				lastPrice = s.midPrice
			}
			s.lock.RUnlock()
			return lastPrice
		}
	}

}

func (s *Strategy) Draw(time types.Time, priceLine types.SeriesExtend, profit types.Series, cumProfit types.Series, zeroPoints types.Series) {
	canvas := types.NewCanvas(s.InstanceID(), s.Interval)
	Length := priceLine.Length()
	if Length > 300 {
		Length = 300
	}
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
	f, err := os.Create(s.CanvasPath)
	if err != nil {
		log.WithError(err).Errorf("cannot create on %s", s.CanvasPath)
		return
	}
	defer f.Close()
	if err := canvas.Render(chart.PNG, f); err != nil {
		log.WithError(err).Errorf("cannot render in drift")
	}

	canvas = types.NewCanvas(s.InstanceID())
	if s.GraphPNLDeductFee {
		canvas.PlotRaw("pnl % (with Fee Deducted)", profit, profit.Length())
	} else {
		canvas.PlotRaw("pnl %", profit, profit.Length())
	}
	f, err = os.Create(s.GraphPNLPath)
	if err != nil {
		log.WithError(err).Errorf("open pnl")
		return
	}
	defer f.Close()
	if err := canvas.Render(chart.PNG, f); err != nil {
		log.WithError(err).Errorf("render pnl")
	}

	canvas = types.NewCanvas(s.InstanceID())
	if s.GraphPNLDeductFee {
		canvas.PlotRaw("cummulative pnl % (with Fee Deducted)", cumProfit, cumProfit.Length())
	} else {
		canvas.PlotRaw("cummulative pnl %", cumProfit, cumProfit.Length())
	}
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

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	instanceID := s.InstanceID()
	// Will be set by persistence if there's any from DB
	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
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

	// Calculate fee-price protection factor to prevent order rejection, especially on FTX.
	// buyOrderFeePriceFactor = 1 + taker_fee * (1 + delta)
	// while placing buy order: quantity = quote_balance / (price * buyOrderFeePriceFactor)
	s.buyOrderFeePriceFactor = fixedpoint.One
	if s.OrderFeeProtection && session.TakerFeeRate.Sign() > 0 {
		safetyRate := session.TakerFeeRate.Mul(fixedpoint.One.Add(Delta))
		s.buyOrderFeePriceFactor = s.buyOrderFeePriceFactor.Add(safetyRate)
	}

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

	// Exit methods from config
	for _, method := range s.ExitMethods {
		method.Bind(session, s.GeneralOrderExecutor)
	}
	buyPrice := fixedpoint.Zero
	sellPrice := fixedpoint.Zero
	Volume := fixedpoint.Zero
	profit := types.Float64Slice{}
	cumProfit := types.Float64Slice{1.}
	orderTagHistory := make(map[uint64]string)
	s.buyPrice = 0
	s.sellPrice = 0
	s.Session.UserDataStream.OnOrderUpdate(func(order types.Order) {
		orderTagHistory[order.OrderID] = order.Tag
	})
	modify := func(p fixedpoint.Value) fixedpoint.Value {
		return p
	}
	if s.GraphPNLDeductFee {
		modify = func(p fixedpoint.Value) fixedpoint.Value {
			return p.Mul(fixedpoint.One.Sub(Fee))
		}
	}
	s.Session.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
		tag, ok := orderTagHistory[trade.OrderID]
		if !ok {
			panic(fmt.Sprintf("cannot find order: %v", trade))
		}
		if tag == "close" {
			if !buyPrice.IsZero() {
				profit.Update(modify(trade.Price.Div(buyPrice)).
					Sub(fixedpoint.One).
					Mul(trade.Quantity).
					Div(Volume).
					Add(fixedpoint.One).
					Float64())
				cumProfit.Update(cumProfit.Last() * profit.Last())
				Volume = Volume.Sub(trade.Quantity)
				if Volume.IsZero() {
					buyPrice = fixedpoint.Zero
				}
				if !sellPrice.IsZero() {
					panic("sellprice shouldn't be zero")
				}
			} else if !sellPrice.IsZero() {
				profit.Update(modify(sellPrice.Div(trade.Price)).
					Sub(fixedpoint.One).
					Mul(trade.Quantity).
					Div(Volume).
					Neg().
					Add(fixedpoint.One).
					Float64())
				cumProfit.Update(cumProfit.Last() * profit.Last())
				Volume = Volume.Add(trade.Quantity)
				if Volume.IsZero() {
					sellPrice = fixedpoint.Zero
				}
				if !buyPrice.IsZero() {
					panic("buyprice shouldn't be zero")
				}
			} else {
				panic("no price available")
			}
		} else if tag == "short" {
			if buyPrice.IsZero() {
				if !sellPrice.IsZero() {
					sellPrice = sellPrice.Mul(Volume).Sub(trade.Price.Mul(trade.Quantity)).Div(Volume.Sub(trade.Quantity))
				} else {
					sellPrice = trade.Price
				}
			} else {
				profit.Update(modify(trade.Price.Div(buyPrice)).Float64())
				cumProfit.Update(cumProfit.Last() * profit.Last())
				buyPrice = fixedpoint.Zero
				Volume = fixedpoint.Zero
				sellPrice = trade.Price
			}
			Volume = Volume.Sub(trade.Quantity)
		} else if tag == "long" {
			if sellPrice.IsZero() {
				if !buyPrice.IsZero() {
					buyPrice = buyPrice.Mul(Volume).Add(trade.Price.Mul(trade.Quantity)).Div(Volume.Add(trade.Quantity))
				} else {
					buyPrice = trade.Price
				}
			} else {
				profit.Update(modify(sellPrice.Div(trade.Price)).Float64())
				cumProfit.Update(cumProfit.Last() * profit.Last())
				sellPrice = fixedpoint.Zero
				buyPrice = trade.Price
				Volume = fixedpoint.Zero
			}
			Volume = Volume.Add(trade.Quantity)
		}
		s.buyPrice = buyPrice.Float64()
		s.highestPrice = s.buyPrice
		s.sellPrice = sellPrice.Float64()
		s.lowestPrice = s.sellPrice
	})

	if err := s.initIndicators(); err != nil {
		log.WithError(err).Errorf("initIndicator failed")
		return nil
	}
	s.initTickerFunctions(ctx)

	dynamicKLine := &types.KLine{}
	priceLine := types.NewQueue(300)
	zeroPoints := types.NewQueue(300)
	stoploss := s.StopLoss.Float64()

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if s.Status != types.StrategyStatusRunning {
			return
		}
		if kline.Symbol != s.Symbol {
			return
		}
		var driftPred, atr float64
		var drift []float64

		if !kline.Closed {
			return
		}
		if kline.Interval == types.Interval1m {
			if s.NoTrailingStopLoss || !s.IsBackTesting() {
				return
			}
			// for doing the trailing stoploss during backtesting
			atr = s.atr.Last()
			price := s.getLastPrice()
			pricef := price.Float64()
			lowf := math.Min(kline.Low.Float64(), pricef)
			highf := math.Max(kline.High.Float64(), pricef)
			d := s.drift.TestUpdate(pricef)
			drift := d.Last()
			ddrift := d.drift.Last()

			if s.lowestPrice > 0 && lowf < s.lowestPrice {
				s.lowestPrice = lowf
			}
			if s.highestPrice > 0 && highf > s.highestPrice {
				s.highestPrice = highf
			}
			avg := s.buyPrice + s.sellPrice

			if s.GeneralOrderExecutor.ActiveMakerOrders().NumOfOrders() > 0 {
				return
			}

			takeProfitFactor := s.takeProfitFactor.Predict(2)
			exitShortCondition := ( /*avg+atr/2 <= highf || avg*(1.+stoploss) <= pricef ||*/ (drift > 0 && ddrift > 0.6) || avg-atr*takeProfitFactor >= pricef ||
				((highf-s.lowestPrice)/s.lowestPrice > 0.003 && (avg-s.lowestPrice)/s.lowestPrice > 0.015)) &&
				(s.Position.IsShort() && !s.Position.IsDust(price))
			exitLongCondition := ( /*avg-atr/2 >= lowf || avg*(1.-stoploss) >= pricef || */ (drift < 0 && ddrift < -0.6) || avg+atr*takeProfitFactor <= pricef ||
				((s.highestPrice-lowf)/lowf > 0.003 && (s.highestPrice-avg)/avg > 0.015)) &&
				(s.Position.IsLong() && !s.Position.IsDust(price))
			if exitShortCondition || exitLongCondition {
				if exitLongCondition && s.highestPrice > avg {
					s.takeProfitFactor.Update((s.highestPrice - avg) / atr * 4)
				} else if exitShortCondition && avg > s.lowestPrice {
					s.takeProfitFactor.Update((avg - s.lowestPrice) / atr * 4)
				}
				_ = s.ClosePosition(ctx, fixedpoint.One)
			}
			return
		}
		dynamicKLine.Set(&kline)

		source := s.getSource(dynamicKLine)
		sourcef := source.Float64()
		priceLine.Update(sourcef)
		s.ma.Update(sourcef)
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
		if s.lowestPrice > 0 && lowf < s.lowestPrice {
			s.lowestPrice = lowf
		}
		if s.highestPrice > 0 && highf > s.highestPrice {
			s.highestPrice = highf
		}
		avg := s.buyPrice + s.sellPrice
		takeProfitFactor := s.takeProfitFactor.Predict(2)

		if !s.IsBackTesting() {
			balances := s.Session.GetAccount().Balances()
			bbgo.Notify("zeroPoint: %.4f, source: %.4f, price: %.4f, driftPred: %.4f, drift: %.4f, drift[1]: %.4f, atr: %.4f, avg: %.4f",
				zeroPoint, sourcef, pricef, driftPred, drift[0], drift[1], atr, avg)
			// Notify will parse args to strings and process separately
			bbgo.Notify("balances: [Base] %s [Quote] %s", balances[s.Market.BaseCurrency].String(), balances[s.Market.QuoteCurrency].String())
		}

		//shortCondition := (sourcef <= zeroPoint && driftPred <= drift[0] && drift[0] <= 0 && drift[1] > 0 && drift[2] > drift[1])
		//longCondition := (sourcef >= zeroPoint && driftPred >= drift[0] && drift[0] >= 0 && drift[1] < 0 && drift[2] < drift[1])
		//bothUp := ddrift[1] < ddrift[0] && drift[1] < drift[0]
		//bothDown := ddrift[1] > ddrift[0] && drift[1] > drift[0]
		shortCondition := (drift[1] >= -0.9 || ddrift[1] >= 0) && (driftPred <= -0.6 || ddriftPred <= 0)
		longCondition := (drift[1] <= 0.9 || ddrift[1] <= 0) && (driftPred >= 0.6 || ddriftPred >= 0)
		exitShortCondition := ((drift[0] >= 0.6 && ddrift[0] >= 0) ||
			avg*(1.+stoploss) <= pricef ||
			avg-atr*takeProfitFactor >= pricef) &&
			s.Position.IsShort() && !longCondition && !shortCondition
		exitLongCondition := ((drift[0] <= -0.6 && ddrift[0] <= 0) ||
			avg*(1.-stoploss) >= pricef ||
			avg+atr*takeProfitFactor <= pricef) &&
			s.Position.IsLong() && !shortCondition && !longCondition

		if (exitShortCondition || exitLongCondition) && s.Position.IsOpened(price) {
			if err := s.GeneralOrderExecutor.GracefulCancel(ctx); err != nil {
				log.WithError(err).Errorf("cannot cancel orders")
				return
			}
			if exitShortCondition && avg > s.lowestPrice {
				s.takeProfitFactor.Update((avg - s.lowestPrice) / atr * 4)
			} else if exitLongCondition && avg < s.highestPrice {
				s.takeProfitFactor.Update((s.highestPrice - avg) / atr * 4)
			}
			if s.takeProfitFactor.Last() == 0 {
				log.Errorf("exit %f %f %f %v", s.highestPrice, s.lowestPrice, avg, s.takeProfitFactor.Array(10))
			}
			_ = s.ClosePosition(ctx, fixedpoint.One)
			return
		}
		if shortCondition {
			if err := s.GeneralOrderExecutor.GracefulCancel(ctx); err != nil {
				log.WithError(err).Errorf("cannot cancel orders")
				return
			}
			baseBalance, ok := s.Session.GetAccount().Balance(s.Market.BaseCurrency)
			if !ok {
				log.Errorf("unable to get baseBalance")
				return
			}
			source = source.Add(fixedpoint.NewFromFloat(s.stdevHigh.Last() * s.HighLowVarianceMultiplier))
			if source.Compare(price) < 0 {
				source = price
			}
			sourcef = source.Float64()

			if s.Market.IsDustQuantity(baseBalance.Available, source) {
				return
			}
			if avg < s.highestPrice && avg > 0 && s.Position.IsLong() {
				s.takeProfitFactor.Update((s.highestPrice - avg) / atr * 4)
				if s.takeProfitFactor.Last() == 0 {
					log.Errorf("short %f %f", s.highestPrice, avg)
				}
			}
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
			orderTagHistory[createdOrders[0].OrderID] = "short"
		}
		if longCondition {
			if err := s.GeneralOrderExecutor.GracefulCancel(ctx); err != nil {
				log.WithError(err).Errorf("cannot cancel orders")
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
				return
			}
			if s.Market.IsDustQuantity(
				quoteBalance.Available.Div(source), source) {
				return
			}
			if avg > s.lowestPrice && s.Position.IsShort() {
				s.takeProfitFactor.Update((avg - s.lowestPrice) / atr * 4)
				if s.takeProfitFactor.Last() == 0 {
					log.Errorf("long %f %f", s.lowestPrice, avg)
				}

			}
			quantity := quoteBalance.Available.Div(source.Mul(s.buyOrderFeePriceFactor))
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
			orderTagHistory[createdOrders[0].OrderID] = "long"
		}
	})

	bbgo.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {

		defer s.Print(os.Stdout)

		defer fmt.Fprintln(os.Stdout, s.TradeStats.BriefString())

		if s.GenerateGraph {
			s.Draw(dynamicKLine.StartTime, priceLine, &profit, &cumProfit, zeroPoints)
		}

		wg.Done()
	})
	return nil
}
