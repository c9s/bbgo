package drift

import (
	"context"
	"encoding/json"
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

	drift    *indicator.Drift
	atr      *indicator.ATR
	midPrice fixedpoint.Value
	lock     sync.RWMutex

	Source        string           `json:"source"`
	Stoploss      fixedpoint.Value `json:"stoploss"`
	CanvasPath    string           `json:"canvasPath"`
	PredictOffset int              `json:"predictOffset"`

	ExitMethods bbgo.ExitMethodSet `json:"exits"`
	Session     *bbgo.ExchangeSession
	*bbgo.GeneralOrderExecutor

	getLastPrice func() fixedpoint.Value
}

func (s *Strategy) Print() {
	b, _ := json.MarshalIndent(s.ExitMethods, "  ", "  ")
	hiyellow := color.New(color.FgHiYellow).FprintfFunc()
	hiyellow(os.Stderr, "------ %s Settings ------\n", s.InstanceID())
	hiyellow(os.Stderr, "canvasPath: %s\n", s.CanvasPath)
	hiyellow(os.Stderr, "source: %s\n", s.Source)
	hiyellow(os.Stderr, "stoploss: %v\n", s.Stoploss)
	hiyellow(os.Stderr, "predictOffset: %d\n", s.PredictOffset)
	hiyellow(os.Stderr, "exits:\n %s\n", string(b))
	hiyellow(os.Stderr, "symbol: %s\n", s.Symbol)
	hiyellow(os.Stderr, "interval: %s\n", s.Interval)
	hiyellow(os.Stderr, "window: %d\n", s.Window)
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
		session.Subscribe(types.MarketTradeChannel, s.Symbol, types.SubscribeOptions{})
	}
	s.ExitMethods.SetAndSubscribe(session, s)
}

func (s *Strategy) ClosePosition(ctx context.Context) (*types.Order, bool) {
	order := s.Position.NewMarketCloseOrder(fixedpoint.One)
	if order == nil {
		return nil, false
	}
	order.TimeInForce = ""
	balances := s.Session.GetAccount().Balances()
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
			return nil, true
		}
		createdOrders, err := s.GeneralOrderExecutor.SubmitOrders(ctx, *order)
		if err != nil {
			order.Quantity = order.Quantity.Mul(fixedpoint.One.Sub(Delta))
			continue
		}
		return &createdOrders[0], true
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
		return func(kline *types.KLine) fixedpoint.Value {
			log.Infof("source not set, use hl2 by default")
			return kline.High.Add(kline.Low).Div(Two)
		}
	default:
		panic(fmt.Sprintf("Unable to parse: %s", s.Source))
	}
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	instanceID := s.InstanceID()
	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
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
		_, _ = s.ClosePosition(ctx)
	})

	s.Session = session
	s.GeneralOrderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.GeneralOrderExecutor.BindEnvironment(s.Environment)
	s.GeneralOrderExecutor.BindProfitStats(s.ProfitStats)
	s.GeneralOrderExecutor.BindTradeStats(s.TradeStats)
	s.GeneralOrderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(s)
	})
	s.GeneralOrderExecutor.Bind()
	for _, method := range s.ExitMethods {
		method.Bind(session, s.GeneralOrderExecutor)
	}

	store, _ := session.MarketDataStore(s.Symbol)

	getSource := s.SourceFuncGenerator()

	s.drift = &indicator.Drift{
		MA:             &indicator.SMA{IntervalWindow: s.IntervalWindow},
		IntervalWindow: s.IntervalWindow,
	}
	s.atr = &indicator.ATR{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: 14}}

	klines, ok := store.KLinesOfInterval(s.Interval)
	if !ok {
		log.Errorf("klines not exists")
		return nil
	}

	dynamicKLine := &types.KLine{}
	for _, kline := range *klines {
		source := getSource(&kline).Float64()
		s.drift.Update(source)
		s.atr.Update(kline.High.Float64(), kline.Low.Float64(), kline.Close.Float64())
	}

	if s.Environment.IsBackTesting() {
		s.getLastPrice = func() fixedpoint.Value {
			lastPrice, ok := s.Session.LastPrice(s.Symbol)
			if !ok {
				log.Error("cannot get lastprice")
			}
			return lastPrice
		}
	} else {
		session.MarketDataStream.OnBookTickerUpdate(func(ticker types.BookTicker) {
			bestBid := ticker.Buy
			bestAsk := ticker.Sell

			if util.TryLock(&s.lock) {
				if !bestAsk.IsZero() && !bestBid.IsZero() {
					s.midPrice = bestAsk.Add(bestBid).Div(Two)
				} else if !bestAsk.IsZero() {
					s.midPrice = bestAsk
				} else {
					s.midPrice = bestBid
				}
				s.lock.Unlock()
			}
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

	priceLine := types.NewQueue(100)
	stoploss := s.Stoploss.Float64()

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
			return
		}
		dynamicKLine.Copy(&kline)

		source := getSource(dynamicKLine)
		sourcef := source.Float64()
		priceLine.Update(sourcef)
		s.drift.Update(sourcef)
		drift = s.drift.Array(2)
		driftPred = s.drift.Predict(s.PredictOffset)
		atr = s.atr.Last()
		price := s.getLastPrice()
		pricef := price.Float64()
		avg := s.Position.AverageCost.Float64()

		shortCondition := (driftPred <= 0 && drift[0] <= 0)
		longCondition := (driftPred >= 0 && drift[0] >= 0)
		exitShortCondition := ((drift[1] < 0 && drift[0] >= 0) || avg+atr/2 <= pricef || avg*(1.+stoploss) <= pricef) &&
			(!s.Position.IsClosed() && !s.Position.IsDust(fixedpoint.Max(price, source))) && !longCondition
		exitLongCondition := ((drift[1] > 0 && drift[0] < 0) || avg-atr/2 >= pricef || avg*(1.-stoploss) >= pricef) &&
			(!s.Position.IsClosed() && !s.Position.IsDust(fixedpoint.Min(price, source))) && !shortCondition

		if exitShortCondition || exitLongCondition {
			if err := s.GeneralOrderExecutor.GracefulCancel(ctx); err != nil {
				log.WithError(err).Errorf("cannot cancel orders")
				return
			}
			_, _ = s.ClosePosition(ctx)
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
			if source.Compare(price) < 0 {
				source = price
			}

			if s.Market.IsDustQuantity(baseBalance.Available, source) {
				return
			}
			_, err := s.GeneralOrderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:    s.Symbol,
				Side:      types.SideTypeSell,
				Type:      types.OrderTypeLimitMaker,
				Price:     source,
				StopPrice: fixedpoint.NewFromFloat(math.Min(sourcef+atr/2, sourcef*(1.+stoploss))),
				Quantity:  baseBalance.Available,
			})
			if err != nil {
				log.WithError(err).Errorf("cannot place sell order")
				return
			}
		}
		if longCondition {
			if err := s.GeneralOrderExecutor.GracefulCancel(ctx); err != nil {
				log.WithError(err).Errorf("cannot cancel orders")
				return
			}
			if source.Compare(price) > 0 {
				source = price
			}
			quoteBalance, ok := s.Session.GetAccount().Balance(s.Market.QuoteCurrency)
			if !ok {
				log.Errorf("unable to get quoteCurrency")
				return
			}
			if s.Market.IsDustQuantity(
				quoteBalance.Available.Div(source), source) {
				return
			}
			_, err := s.GeneralOrderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:    s.Symbol,
				Side:      types.SideTypeBuy,
				Type:      types.OrderTypeLimitMaker,
				Price:     source,
				StopPrice: fixedpoint.NewFromFloat(math.Max(sourcef-atr/2, sourcef*(1.-stoploss))),
				Quantity:  quoteBalance.Available.Div(source),
			})
			if err != nil {
				log.WithError(err).Errorf("cannot place buy order")
				return
			}
		}
	})

	bbgo.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		_, _ = fmt.Fprintln(os.Stderr, s.TradeStats.String())
		s.Print()
		canvas := types.NewCanvas(s.InstanceID(), s.Interval)
		mean := priceLine.Mean(100)
		highestPrice := priceLine.Minus(mean).Highest(100)
		highestDrift := s.drift.Highest(100)
		ratio := highestDrift / highestPrice
		canvas.Plot("drift", s.drift, dynamicKLine.StartTime, 100)
		canvas.Plot("zero", types.NumberSeries(0), dynamicKLine.StartTime, 100)
		canvas.Plot("price", priceLine.Minus(mean).Mul(ratio), dynamicKLine.StartTime, 100)
		f, err := os.Create(s.CanvasPath)
		if err != nil {
			log.Errorf("%v cannot create on %s", err, s.CanvasPath)
		}
		defer f.Close()
		canvas.Render(chart.PNG, f)
		wg.Done()
	})
	return nil
}
