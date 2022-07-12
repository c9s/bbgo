package drift

import (
	"context"
	"fmt"
	"os"
	"sync"

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

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Symbol string `json:"symbol"`

	bbgo.StrategyController
	types.Market
	types.IntervalWindow

	*bbgo.Environment
	*types.Position
	*types.ProfitStats
	*types.TradeStats

	drift    *indicator.Drift
	atr      *indicator.ATR
	midPrice fixedpoint.Value
	lock     sync.RWMutex

	stoploss float64 `json:"stoploss"`

	ExitMethods bbgo.ExitMethodSet `json:"exits"`
	Session     *bbgo.ExchangeSession
	*bbgo.GeneralOrderExecutor
	*bbgo.ActiveOrderBook
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

var Three fixedpoint.Value = fixedpoint.NewFromInt(3)
var Two fixedpoint.Value = fixedpoint.NewFromInt(2)

func (s *Strategy) GetLastPrice() (lastPrice fixedpoint.Value) {
	var ok bool
	if s.Environment.IsBackTesting() {
		lastPrice, ok = s.Session.LastPrice(s.Symbol)
		if !ok {
			log.Error("cannot get lastprice")
			return lastPrice
		}
	} else {
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
	}
	return lastPrice
}

var Delta fixedpoint.Value = fixedpoint.NewFromFloat(0.01)

func (s *Strategy) ClosePosition(ctx context.Context) (*types.Order, bool) {
	order := s.Position.NewMarketCloseOrder(fixedpoint.One)
	if order == nil {
		return nil, false
	}
	order.TimeInForce = ""
	balances := s.Session.GetAccount().Balances()
	baseBalance := balances[s.Market.BaseCurrency].Available
	price := s.GetLastPrice()
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
	s.ActiveOrderBook = bbgo.NewActiveOrderBook(s.Symbol)
	s.ActiveOrderBook.BindStream(session.UserDataStream)

	store, _ := session.MarketDataStore(s.Symbol)

	getSource := func(kline *types.KLine) fixedpoint.Value {
		//return kline.High.Add(kline.Low).Div(Two)
		//return kline.Close
		return kline.High.Add(kline.Low).Add(kline.Close).Div(Three)
	}

	s.drift = &indicator.Drift{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.Window}}
	s.atr = &indicator.ATR{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: 14}}

	klines, ok := store.KLinesOfInterval(s.Interval)
	if !ok {
		log.Errorf("klines not exists")
		return nil
	}
	for _, kline := range *klines {
		source := getSource(&kline).Float64()
		s.drift.Update(source)
		s.atr.Update(kline.High.Float64(), kline.Low.Float64(), kline.Close.Float64())
	}

	session.MarketDataStream.OnBookTickerUpdate(func(ticker types.BookTicker) {
		if s.Environment.IsBackTesting() {
			return
		}
		bestBid := ticker.Buy
		bestAsk := ticker.Sell

		if util.TryLock(&s.lock) {
			if !bestAsk.IsZero() && !bestBid.IsZero() {
				s.midPrice = bestAsk.Add(bestBid).Div(types.Two)
			} else if !bestAsk.IsZero() {
				s.midPrice = bestAsk
			} else {
				s.midPrice = bestBid
			}
			s.lock.Unlock()
		}
	})

	dynamicKLine := &types.KLine{}
	priceLine := types.NewQueue(100)

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
		dynamicKLine.Closed = false
		s.drift.Update(sourcef)
		drift = s.drift.Array(2)
		driftPred = s.drift.Predict(3)
		atr = s.atr.Last()
		price := s.GetLastPrice()
		avg := s.Position.AverageCost.Float64()

		shortCondition := (driftPred <= 0 && drift[0] <= 0) && (s.Position.IsClosed() || s.Position.IsDust(fixedpoint.Max(price, source)))
		longCondition := (driftPred >= 0 && drift[0] >= 0) && (s.Position.IsClosed() || s.Position.IsDust(fixedpoint.Min(price, source)))
		exitShortCondition := ((drift[1] < 0 && drift[0] >= 0) || avg+atr/2 <= price.Float64() || avg*(1.+s.stoploss) <= price.Float64()) &&
			(!s.Position.IsClosed() && !s.Position.IsDust(fixedpoint.Max(price, source)))
		exitLongCondition := ((drift[1] > 0 && drift[0] < 0) || avg-atr/2 >= price.Float64() || avg*(1.-s.stoploss) >= price.Float64()) &&
			(!s.Position.IsClosed() && !s.Position.IsDust(fixedpoint.Min(price, source)))

		if shortCondition {
			if s.ActiveOrderBook.NumOfOrders() > 0 {
				if err := s.GeneralOrderExecutor.GracefulCancelActiveOrderBook(ctx, s.ActiveOrderBook); err != nil {
					log.WithError(err).Errorf("cannot cancel orders")
					return
				}
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
				StopPrice: fixedpoint.NewFromFloat(sourcef + atr/2),
				Quantity:  baseBalance.Available,
			})
			if err != nil {
				log.WithError(err).Errorf("cannot place sell order")
				return
			}
		}
		if exitShortCondition {
			if s.ActiveOrderBook.NumOfOrders() > 0 {
				if err := s.GeneralOrderExecutor.GracefulCancelActiveOrderBook(ctx, s.ActiveOrderBook); err != nil {
					log.WithError(err).Errorf("cannot cancel orders")
					return
				}
			}
			_, _ = s.ClosePosition(ctx)
		}
		if longCondition {
			if s.ActiveOrderBook.NumOfOrders() > 0 {
				if err := s.GeneralOrderExecutor.GracefulCancelActiveOrderBook(ctx, s.ActiveOrderBook); err != nil {
					log.WithError(err).Errorf("cannot cancel orders")
					return
				}
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
			if !s.Position.IsClosed() && !s.Position.IsDust(source) {
				return
			}
			_, err := s.GeneralOrderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:    s.Symbol,
				Side:      types.SideTypeBuy,
				Type:      types.OrderTypeLimitMaker,
				Price:     source,
				StopPrice: fixedpoint.NewFromFloat(sourcef - atr/2),
				Quantity:  quoteBalance.Available.Div(source),
			})
			if err != nil {
				log.WithError(err).Errorf("cannot place buy order")
				return
			}
		}
		if exitLongCondition {
			if s.ActiveOrderBook.NumOfOrders() > 0 {
				if err := s.GeneralOrderExecutor.GracefulCancelActiveOrderBook(ctx, s.ActiveOrderBook); err != nil {
					log.WithError(err).Errorf("cannot cancel orders")
					return
				}
			}
			_, _ = s.ClosePosition(ctx)
		}
	})

	bbgo.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		_, _ = fmt.Fprintln(os.Stderr, s.TradeStats.String())
		canvas := types.NewCanvas(s.InstanceID(), s.Interval)
		fmt.Println(dynamicKLine.StartTime, dynamicKLine.EndTime)
		mean := priceLine.Mean(100)
		highestPrice := priceLine.Highest(100)
		highestDrift := s.drift.Highest(100)
		ratio := highestDrift / highestPrice
		canvas.Plot("drift", s.drift, dynamicKLine.StartTime, 100)
		canvas.Plot("zero", types.NumberSeries(0), dynamicKLine.StartTime, 100)
		canvas.Plot("price", priceLine.Minus(mean).Mul(ratio), dynamicKLine.StartTime, 100)
		f, _ := os.Create("output.png")
		defer f.Close()
		canvas.Render(chart.PNG, f)
		wg.Done()
	})
	return nil
}
