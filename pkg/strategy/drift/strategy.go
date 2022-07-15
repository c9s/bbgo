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

	Source             string           `json:"source"`
	StopLoss           fixedpoint.Value `json:"stoploss"`
	CanvasPath         string           `json:"canvasPath"`
	PredictOffset      int              `json:"predictOffset"`
	NoStopPrice        bool             `json:"noStopPrice"`
	NoTrailingStopLoss bool             `json:"noTrailingStopLoss"`

	// This is not related to trade but for statistics graph generation
	// Will deduct fee in percentage from every trade
	GraphPNLDeductFee bool   `json:"graphPNLDeductFee"`
	GraphPNLPath      string `json:"graphPNLPath"`
	GraphCumPNLPath   string `json:"graphCumPNLPath"`
	// Whether to generate graph when shutdown
	GenerateGraph bool `json:"generateGraph"`

	StopOrders map[uint64]types.SubmitOrder

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
	hiyellow(f, "canvasPath: %s\n", s.CanvasPath)
	hiyellow(f, "source: %s\n", s.Source)
	hiyellow(f, "stoploss: %v\n", s.StopLoss)
	hiyellow(f, "predictOffset: %d\n", s.PredictOffset)
	hiyellow(f, "exits:\n %s\n", string(b))
	hiyellow(f, "symbol: %s\n", s.Symbol)
	hiyellow(f, "interval: %s\n", s.Interval)
	hiyellow(f, "window: %d\n", s.Window)
	hiyellow(f, "noStopPrice: %v\n", s.NoStopPrice)
	hiyellow(f, "noTrailingStopLoss: %v\n", s.NoTrailingStopLoss)
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

func (s *Strategy) ClosePosition(ctx context.Context) (*types.Order, bool) {
	order := s.Position.NewMarketCloseOrder(fixedpoint.One)
	if order == nil {
		return nil, false
	}
	order.Tag = "close"
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

func (s *Strategy) BindStopLoss(ctx context.Context) {
	s.StopOrders = make(map[uint64]types.SubmitOrder)
	s.Session.UserDataStream.OnOrderUpdate(func(order types.Order) {
		if len(s.StopOrders) == 0 {
			return
		}
		if order.Symbol != s.Symbol {
			return
		}
		if order.Status == types.OrderStatusCanceled {
			delete(s.StopOrders, order.OrderID)
			return
		}
		if order.Status != types.OrderStatusFilled {
			return
		}
		if o, ok := s.StopOrders[order.OrderID]; ok {
			delete(s.StopOrders, order.OrderID)
			if o.Side == types.SideTypeBuy {
				quoteBalance, ok := s.Session.GetAccount().Balance(s.Market.QuoteCurrency)
				if !ok {
					log.Errorf("unable to get quoteCurrency")
					return
				}
				o.Quantity = quoteBalance.Available.Div(o.Price)
			} else {
				baseBalance, ok := s.Session.GetAccount().Balance(s.Market.BaseCurrency)
				if !ok {
					log.Errorf("unable to get baseCurrency")
					return
				}
				o.Quantity = baseBalance.Available
			}
			if _, err := s.GeneralOrderExecutor.SubmitOrders(ctx, o); err != nil {
				log.WithError(err).Errorf("cannot send stop order: %v", order)
			}
		}
	})
}

func (s *Strategy) InitIndicators() error {
	s.drift = &indicator.Drift{
		MA:             &indicator.SMA{IntervalWindow: s.IntervalWindow},
		IntervalWindow: s.IntervalWindow,
	}
	s.atr = &indicator.ATR{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: 14}}
	store, _ := s.Session.MarketDataStore(s.Symbol)
	klines, ok := store.KLinesOfInterval(s.Interval)
	if !ok {
		return errors.New("klines not exists")
	}

	for _, kline := range *klines {
		source := s.getSource(&kline).Float64()
		s.drift.Update(source)
		s.atr.PushK(kline)
	}
	return nil
}

func (s *Strategy) InitTickerFunctions(ctx context.Context) {
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

			var pricef, stoploss, atr, avg float64
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
				s.lock.Unlock()
			} else {
				return
			}

			// for trailing stoploss during the realtime
			if s.NoTrailingStopLoss {
				return
			}
			atr = s.atr.Last()
			avg = s.Position.AverageCost.Float64()
			stoploss = s.StopLoss.Float64()
			exitShortCondition := (avg+atr/2 <= pricef || avg*(1.+stoploss) <= pricef) &&
				(!s.Position.IsClosed() && !s.Position.IsDust(price))
			exitLongCondition := (avg-atr/2 >= pricef || avg*(1.-stoploss) >= pricef) &&
				(!s.Position.IsClosed() && !s.Position.IsDust(price))
			if exitShortCondition || exitLongCondition {
				if err := s.GeneralOrderExecutor.GracefulCancel(ctx); err != nil {
					log.WithError(err).Errorf("cannot cancel orders")
					return
				}
				// Cleanup pending StopOrders
				s.StopOrders = make(map[uint64]types.SubmitOrder)
				_, _ = s.ClosePosition(ctx)
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

}

func (s *Strategy) Draw(time types.Time, priceLine types.SeriesExtend, profit types.Series, cumProfit types.Series) {
	canvas := types.NewCanvas(s.InstanceID(), s.Interval)
	Length := priceLine.Length()
	if Length > 100 {
		Length = 100
	}
	mean := priceLine.Mean(Length)
	highestPrice := priceLine.Minus(mean).Abs().Highest(Length)
	highestDrift := s.drift.Abs().Highest(Length)
	meanDrift := s.drift.Mean(Length)
	ratio := highestDrift / highestPrice
	canvas.Plot("drift", s.drift, time, Length)
	canvas.Plot("zero", types.NumberSeries(0), time, Length)
	canvas.Plot("price", priceLine.Minus(mean).Mul(ratio), time, Length)
	canvas.Plot("0", types.NumberSeries(meanDrift), time, Length)
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
		panic("open pnl")
	}
	defer f.Close()
	if err := canvas.Render(chart.PNG, f); err != nil {
		panic("render pnl")
	}

	canvas = types.NewCanvas(s.InstanceID())
	if s.GraphPNLDeductFee {
		canvas.PlotRaw("cummulative pnl % (with Fee Deducted)", cumProfit, cumProfit.Length())
	} else {
		canvas.PlotRaw("cummulative pnl %", cumProfit, cumProfit.Length())
	}
	f, err = os.Create(s.GraphCumPNLPath)
	if err != nil {
		panic("open cumpnl")
	}
	defer f.Close()
	if err := canvas.Render(chart.PNG, f); err != nil {
		panic("render cumpnl")
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

	// StrategyController
	s.Status = types.StrategyStatusRunning
	// Get source function from config input
	s.getSource = s.SourceFuncGenerator()

	s.OnSuspend(func() {
		_ = s.GeneralOrderExecutor.GracefulCancel(ctx)
	})

	s.OnEmergencyStop(func() {
		_ = s.GeneralOrderExecutor.GracefulCancel(ctx)
		_, _ = s.ClosePosition(ctx)
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
	profit := types.Float64Slice{}
	cumProfit := types.Float64Slice{1.}
	orderTagHistory := make(map[uint64]string)
	if s.GenerateGraph {
		s.Session.UserDataStream.OnOrderUpdate(func(order types.Order) {
			orderTagHistory[order.OrderID] = order.Tag
		})
		modify := func(p fixedpoint.Value) fixedpoint.Value {
			return p
		}
		if s.GraphPNLDeductFee {
			fee := fixedpoint.NewFromFloat(0.0004) // taker fee % * 2, for upper bound
			modify = func(p fixedpoint.Value) fixedpoint.Value {
				return p.Mul(fixedpoint.One.Sub(fee))
			}
		}
		s.Session.UserDataStream.OnTradeUpdate(func(trade types.Trade) {
			tag, ok := orderTagHistory[trade.OrderID]
			if !ok {
				panic(fmt.Sprintf("cannot find order: %v", trade))
			}
			if tag == "close" {
				if !buyPrice.IsZero() {
					profit.Update(modify(trade.Price.Div(buyPrice)).Float64())
					cumProfit.Update(cumProfit.Last() * profit.Last())
					buyPrice = fixedpoint.Zero
					if !sellPrice.IsZero() {
						panic("sellprice shouldn't be zero")
					}
				} else if !sellPrice.IsZero() {
					profit.Update(modify(sellPrice.Div(trade.Price)).Float64())
					cumProfit.Update(cumProfit.Last() * profit.Last())
					sellPrice = fixedpoint.Zero
					if !buyPrice.IsZero() {
						panic("buyprice shouldn't be zero")
					}
				} else {
					panic("no price available")
				}
			} else if tag == "short" {
				if buyPrice.IsZero() {
					if !sellPrice.IsZero() {
						panic("sellPrice not zero")
					}
					sellPrice = trade.Price
				} else {
					profit.Update(modify(trade.Price.Div(buyPrice)).Float64())
					cumProfit.Update(cumProfit.Last() * profit.Last())
					buyPrice = fixedpoint.Zero
					sellPrice = trade.Price
				}
			} else if tag == "long" {
				if sellPrice.IsZero() {
					if !buyPrice.IsZero() {
						panic("buyPrice not zero")
					}
					buyPrice = trade.Price
				} else {
					profit.Update(modify(sellPrice.Div(trade.Price)).Float64())
					cumProfit.Update(cumProfit.Last() * profit.Last())
					sellPrice = fixedpoint.Zero
					buyPrice = trade.Price
				}
			} else if tag == "sl" {
				if !buyPrice.IsZero() {
					profit.Update(modify(trade.Price.Div(buyPrice)).Float64())
					cumProfit.Update(cumProfit.Last() * profit.Last())
					buyPrice = fixedpoint.Zero
				} else if !sellPrice.IsZero() {
					profit.Update(modify(sellPrice.Div(trade.Price)).Float64())
					cumProfit.Update(cumProfit.Last() * profit.Last())
					sellPrice = fixedpoint.Zero
				} else {
					panic("no position to sl")
				}
			}
		})
	}

	s.BindStopLoss(ctx)

	if err := s.InitIndicators(); err != nil {
		log.WithError(err).Errorf("InitIndicator failed")
		return nil
	}
	s.InitTickerFunctions(ctx)

	dynamicKLine := &types.KLine{}
	priceLine := types.NewQueue(100)
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
			avg := s.Position.AverageCost.Float64()

			exitShortCondition := (avg+atr/2 <= highf || avg*(1.+stoploss) <= highf) &&
				(!s.Position.IsClosed() && !s.Position.IsDust(price))
			exitLongCondition := (avg-atr/2 >= lowf || avg*(1.-stoploss) >= lowf) &&
				(!s.Position.IsClosed() && !s.Position.IsDust(price))
			if exitShortCondition || exitLongCondition {
				if err := s.GeneralOrderExecutor.GracefulCancel(ctx); err != nil {
					log.WithError(err).Errorf("cannot cancel orders")
					return
				}
				// Cleanup pending StopOrders
				s.StopOrders = make(map[uint64]types.SubmitOrder)
				_, _ = s.ClosePosition(ctx)
			}
			return
		}
		dynamicKLine.Copy(&kline)

		source := s.getSource(dynamicKLine)
		sourcef := source.Float64()
		priceLine.Update(sourcef)
		s.drift.Update(sourcef)
		s.atr.PushK(kline)
		drift = s.drift.Array(2)
		driftPred = s.drift.Predict(s.PredictOffset)
		atr = s.atr.Last()
		price := s.getLastPrice()
		pricef := price.Float64()
		lowf := math.Min(kline.Low.Float64(), pricef)
		highf := math.Max(kline.High.Float64(), pricef)
		avg := s.Position.AverageCost.Float64()

		shortCondition := (driftPred <= 0 && drift[0] <= 0)
		longCondition := (driftPred >= 0 && drift[0] >= 0)
		exitShortCondition := ((drift[1] < 0 && drift[0] >= 0) || avg+atr/2 <= highf || avg*(1.+stoploss) <= highf) &&
			(!s.Position.IsClosed() && !s.Position.IsDust(fixedpoint.Max(price, source))) && !longCondition
		exitLongCondition := ((drift[1] > 0 && drift[0] < 0) || avg-atr/2 >= lowf || avg*(1.-stoploss) >= lowf) &&
			(!s.Position.IsClosed() && !s.Position.IsDust(fixedpoint.Min(price, source))) && !shortCondition

		if exitShortCondition || exitLongCondition {
			if err := s.GeneralOrderExecutor.GracefulCancel(ctx); err != nil {
				log.WithError(err).Errorf("cannot cancel orders")
				return
			}
			// Cleanup pending StopOrders
			s.StopOrders = make(map[uint64]types.SubmitOrder)
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
			// Cleanup pending StopOrders
			s.StopOrders = make(map[uint64]types.SubmitOrder)
			quantity := baseBalance.Available
			stopPrice := fixedpoint.NewFromFloat(math.Min(sourcef+atr/2, sourcef*(1.+stoploss)))
			stopOrder := types.SubmitOrder{
				Symbol:    s.Symbol,
				Side:      types.SideTypeBuy,
				Type:      types.OrderTypeStopLimit,
				StopPrice: stopPrice,
				Price:     stopPrice,
				Quantity:  quantity,
				Tag:       "sl",
			}
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
			if s.NoStopPrice {
				return
			}
			if createdOrders[0].Status == types.OrderStatusFilled {
				s.GeneralOrderExecutor.SubmitOrders(ctx, stopOrder)
				return
			}
			s.StopOrders[createdOrders[0].OrderID] = stopOrder
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
			// Cleanup pending StopOrders
			s.StopOrders = make(map[uint64]types.SubmitOrder)
			quantity := quoteBalance.Available.Div(source)
			stopPrice := fixedpoint.NewFromFloat(math.Max(sourcef-atr/2, sourcef*(1.-stoploss)))
			stopOrder := types.SubmitOrder{
				Symbol:      s.Symbol,
				Side:        types.SideTypeSell,
				Type:        types.OrderTypeStopLimit,
				TimeInForce: types.TimeInForceGTC,
				StopPrice:   stopPrice,
				Price:       stopPrice,
				Quantity:    quantity,
				Tag:         "sl",
			}
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
			if s.NoStopPrice {
				return
			}
			if createdOrders[0].Status == types.OrderStatusFilled {
				s.GeneralOrderExecutor.SubmitOrders(ctx, stopOrder)
				return
			}
			s.StopOrders[createdOrders[0].OrderID] = stopOrder
		}
	})

	bbgo.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {

		defer s.Print(os.Stdout)

		defer fmt.Fprintln(os.Stdout, s.TradeStats.String())

		if s.GenerateGraph {
			s.Draw(dynamicKLine.StartTime, priceLine, &profit, &cumProfit)
		}

		wg.Done()
	})
	return nil
}
