package harmonic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/interact"
	"github.com/c9s/bbgo/pkg/types"

	"github.com/sirupsen/logrus"
	"github.com/wcharczuk/go-chart/v2"
)

const ID = "harmonic"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Environment *bbgo.Environment
	Symbol      string `json:"symbol"`
	Market      types.Market

	types.IntervalWindow
	//bbgo.OpenPositionOptions

	// persistence fields
	Position    *types.Position    `persistence:"position"`
	ProfitStats *types.ProfitStats `persistence:"profit_stats"`
	TradeStats  *types.TradeStats  `persistence:"trade_stats"`

	ExitMethods bbgo.ExitMethodSet `json:"exits"`

	session       *bbgo.ExchangeSession
	orderExecutor *bbgo.GeneralOrderExecutor

	bbgo.QuantityOrAmount

	// StrategyController
	bbgo.StrategyController

	shark *SHARK

	// plotting
	bbgo.SourceSelector
	priceLines     *types.Queue
	midPrice       fixedpoint.Value
	lock           sync.RWMutex `ignore:"true"`
	positionLock   sync.RWMutex `ignore:"true"`
	startTime      time.Time
	minutesCounter int
	frameKLine     *types.KLine
	kline1m        *types.KLine
	CanvasPath     string `json:"canvasPath"`
	HLRangeWindow  int    `json:"hlRangeWindow"`
	Window1m       int    `json:"window1m"`

	// This is not related to trade but for statistics graph generation
	// Will deduct fee in percentage from every trade
	GraphPNLDeductFee bool   `json:"graphPNLDeductFee"`
	GraphPNLPath      string `json:"graphPNLPath"`
	GraphCumPNLPath   string `json:"graphCumPNLPath"`
	// Whether to generate graph when shutdown
	GenerateGraph bool `json:"generateGraph"`
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})

	if !bbgo.IsBackTesting {
		session.Subscribe(types.MarketTradeChannel, s.Symbol, types.SubscribeOptions{})
	}

	s.ExitMethods.SetAndSubscribe(session, s)
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	var instanceID = s.InstanceID()

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
		// Cancel active orders
		_ = s.orderExecutor.GracefulCancel(ctx)
	})

	s.OnEmergencyStop(func() {
		// Cancel active orders
		_ = s.orderExecutor.GracefulCancel(ctx)
		// Close 100% position
		//_ = s.ClosePosition(ctx, fixedpoint.One)
	})

	s.session = session

	s.orderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.orderExecutor.BindEnvironment(s.Environment)
	s.orderExecutor.BindProfitStats(s.ProfitStats)
	s.orderExecutor.BindTradeStats(s.TradeStats)

	profit := floats.Slice{1., 1.}
	price, _ := s.session.LastPrice(s.Symbol)
	initAsset := s.CalcAssetValue(price).Float64()
	cumProfit := floats.Slice{initAsset, initAsset}
	s.orderExecutor.TradeCollector().OnTrade(func(trade types.Trade, _profit, netProfit fixedpoint.Value) {
		profit.Update(netProfit.Float64())
		cumProfit.Update(s.CalcAssetValue(trade.Price).Float64())
	})
	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(s)
	})
	s.orderExecutor.Bind()

	for _, method := range s.ExitMethods {
		method.Bind(session, s.orderExecutor)
	}

	kLineStore, _ := s.session.MarketDataStore(s.Symbol)
	s.shark = &SHARK{IntervalWindow: types.IntervalWindow{Window: s.Window, Interval: s.Interval}}
	s.shark.BindK(s.session.MarketDataStream, s.Symbol, s.shark.Interval)
	if klines, ok := kLineStore.KLinesOfInterval(s.shark.Interval); ok {
		s.shark.LoadK((*klines)[0:])
	}
	s.session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {

		log.Infof("Shark Score: %f, Current Price: %f", s.shark.Last(), kline.Close.Float64())

		//previousRegime := s.shark.Values.Tail(10).Mean()
		//zeroThreshold := 5.

		if s.shark.Rank(s.Window).Last()/float64(s.Window) > 0.99 { // && ((previousRegime < zeroThreshold && previousRegime > -zeroThreshold) || s.shark.Index(1) < 0)
			if s.Position.IsShort() {
				_ = s.orderExecutor.GracefulCancel(ctx)
				s.orderExecutor.ClosePosition(ctx, fixedpoint.One, "close short position")
			}
			_, err := s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:   s.Symbol,
				Side:     types.SideTypeBuy,
				Quantity: s.Quantity,
				Type:     types.OrderTypeMarket,
				Tag:      "shark long: buy in",
			})
			if err == nil {
				_, err = s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
					Symbol:   s.Symbol,
					Side:     types.SideTypeSell,
					Quantity: s.Quantity,
					Price:    fixedpoint.NewFromFloat(s.shark.Highs.Tail(100).Max()),
					Type:     types.OrderTypeLimit,
					Tag:      "shark long: sell back",
				})
			}
			if err != nil {
				log.Errorln(err)
			}

		} else if s.shark.Rank(s.Window).Last()/float64(s.Window) < 0.01 { // && ((previousRegime < zeroThreshold && previousRegime > -zeroThreshold) || s.shark.Index(1) > 0)
			if s.Position.IsLong() {
				_ = s.orderExecutor.GracefulCancel(ctx)
				s.orderExecutor.ClosePosition(ctx, fixedpoint.One, "close long position")
			}
			_, err := s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:   s.Symbol,
				Side:     types.SideTypeSell,
				Quantity: s.Quantity,
				Type:     types.OrderTypeMarket,
				Tag:      "shark short: sell in",
			})
			if err == nil {
				_, err = s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
					Symbol:   s.Symbol,
					Side:     types.SideTypeBuy,
					Quantity: s.Quantity,
					Price:    fixedpoint.NewFromFloat(s.shark.Lows.Tail(100).Min()),
					Type:     types.OrderTypeLimit,
					Tag:      "shark short: buy back",
				})
			}
			if err != nil {
				log.Errorln(err)
			}
		}
	}))

	bbgo.RegisterCommand("/draw", "Draw Indicators", func(reply interact.Reply) {
		canvas := s.DrawIndicators(s.frameKLine.StartTime)
		var buffer bytes.Buffer
		if err := canvas.Render(chart.PNG, &buffer); err != nil {
			log.WithError(err).Errorf("cannot render indicators in oneliner")
			reply.Message(fmt.Sprintf("[error] cannot render indicators in harmonic: %v", err))
			return
		}
		bbgo.SendPhoto(&buffer)
	})

	bbgo.RegisterCommand("/pnl", "Draw PNL(%) per trade", func(reply interact.Reply) {
		canvas := s.DrawPNL(&profit)
		var buffer bytes.Buffer
		if err := canvas.Render(chart.PNG, &buffer); err != nil {
			log.WithError(err).Errorf("cannot render pnl in oneliner")
			reply.Message(fmt.Sprintf("[error] cannot render pnl in harmonic: %v", err))
			return
		}
		bbgo.SendPhoto(&buffer)
	})

	bbgo.RegisterCommand("/cumpnl", "Draw Cumulative PNL(Quote)", func(reply interact.Reply) {
		canvas := s.DrawCumPNL(&cumProfit)
		var buffer bytes.Buffer
		if err := canvas.Render(chart.PNG, &buffer); err != nil {
			log.WithError(err).Errorf("cannot render cumpnl in oneliner")
			reply.Message(fmt.Sprintf("[error] canot render cumpnl in harmonic: %v", err))
			return
		}
		bbgo.SendPhoto(&buffer)
	})

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		_, _ = fmt.Fprintln(os.Stderr, s.TradeStats.String())
		_ = s.orderExecutor.GracefulCancel(ctx)
	})

	return nil
}

func (s *Strategy) CalcAssetValue(price fixedpoint.Value) fixedpoint.Value {
	balances := s.session.GetAccount().Balances()
	return balances[s.Market.BaseCurrency].Total().Mul(price).Add(balances[s.Market.QuoteCurrency].Total())
}

func (s *Strategy) DrawPNL(profit types.Series) *types.Canvas {
	canvas := types.NewCanvas(s.InstanceID())
	//log.Errorf("pnl Highest: %f, Lowest: %f", types.Highest(profit, profit.Length()), types.Lowest(profit, profit.Length()))
	length := profit.Length()
	if s.GraphPNLDeductFee {
		canvas.PlotRaw("pnl (with Fee Deducted)", profit, length)
	} else {
		canvas.PlotRaw("pnl", profit, length)
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
	canvas.PlotRaw("cumulative pnl", cumProfit, cumProfit.Length())
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

func (s *Strategy) initIndicators(store *bbgo.SerialMarketDataStore) error {

	klines, ok := store.KLinesOfInterval(s.Interval)
	klinesLength := len(*klines)
	if !ok || klinesLength == 0 {
		return errors.New("klines not exists")
	}
	if s.frameKLine != nil && klines != nil {
		s.frameKLine.Set(&(*klines)[len(*klines)-1])
	}
	klines, ok = store.KLinesOfInterval(types.Interval1m)
	klinesLength = len(*klines)
	if !ok || klinesLength == 0 {
		return errors.New("klines not exists")
	}
	if s.kline1m != nil && klines != nil {
		s.kline1m.Set(&(*klines)[len(*klines)-1])
	}
	s.startTime = s.kline1m.StartTime.Time().Add(s.kline1m.Interval.Duration())
	return nil
}

func (s *Strategy) DrawIndicators(time types.Time) *types.Canvas {
	canvas := types.NewCanvas(s.InstanceID(), s.Interval)
	Length := s.priceLines.Length()
	if Length > 300 {
		Length = 300
	}
	log.Infof("draw indicators with %d data", Length)
	mean := s.priceLines.Mean(Length)

	canvas.Plot("zero", types.NumberSeries(mean), time, Length)
	canvas.Plot("price", s.priceLines, time, Length)
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
		log.WithError(err).Errorf("cannot render in harmonic")
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
