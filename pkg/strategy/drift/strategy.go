package drift

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/sirupsen/logrus"

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

	drift    types.UpdatableSeriesExtend
	atr      *indicator.ATR
	midPrice fixedpoint.Value
	lock     sync.RWMutex

	Session *bbgo.ExchangeSession
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
}

var Three fixedpoint.Value = fixedpoint.NewFromInt(3)

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
		s.TradeStats = &types.TradeStats{}
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
	s.ActiveOrderBook = bbgo.NewActiveOrderBook(s.Symbol)
	s.ActiveOrderBook.BindStream(session.UserDataStream)

	store, _ := session.MarketDataStore(s.Symbol)

	s.drift = &indicator.Drift{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: 5}}
	s.atr = &indicator.ATR{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: 34}}
	s.atr.Bind(store)

	klines, ok := store.KLinesOfInterval(s.Interval)
	if !ok {
		log.Errorf("klines not exists")
		return nil
	}
	for _, kline := range *klines {
		s.drift.Update(kline.High.Add(kline.Low).Add(kline.Close).Div(Three).Float64())
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

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if s.Status != types.StrategyStatusRunning {
			return
		}
		if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
			return
		}
		hlc3 := kline.High.Add(kline.Low).Add(kline.Close).Div(Three)
		s.drift.Update(hlc3.Float64())
		price := s.GetLastPrice()
		if s.drift.Last() < 0 && s.drift.Index(1) >= 0 {
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
			if hlc3.Compare(price) < 0 {
				hlc3 = price
			}

			if s.Market.IsDustQuantity(baseBalance.Available, hlc3) {
				return
			}
			_, err := s.GeneralOrderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:    s.Symbol,
				Side:      types.SideTypeSell,
				Type:      types.OrderTypeLimitMaker,
				Price:     hlc3,
				StopPrice: hlc3.Add(fixedpoint.NewFromFloat(s.atr.Last() / 3)),
				Quantity:  baseBalance.Available,
			})
			if err != nil {
				log.WithError(err).Errorf("cannot place sell order")
				return
			}
		}
		if s.drift.Last() > 0 && s.drift.Index(1) <= 0 {
			if s.ActiveOrderBook.NumOfOrders() > 0 {
				if err := s.GeneralOrderExecutor.GracefulCancelActiveOrderBook(ctx, s.ActiveOrderBook); err != nil {
					log.WithError(err).Errorf("cannot cancel orders")
					return
				}
			}
			if hlc3.Compare(price) > 0 {
				hlc3 = price
			}
			quoteBalance, ok := s.Session.GetAccount().Balance(s.Market.QuoteCurrency)
			if !ok {
				log.Errorf("unable to get quoteCurrency")
				return
			}
			if s.Market.IsDustQuantity(
				quoteBalance.Available.Div(hlc3), hlc3) {
				return
			}
			_, err := s.GeneralOrderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:    s.Symbol,
				Side:      types.SideTypeBuy,
				Type:      types.OrderTypeLimitMaker,
				Price:     hlc3,
				StopPrice: hlc3.Sub(fixedpoint.NewFromFloat(s.atr.Last() / 3)),
				Quantity:  quoteBalance.Available.Div(hlc3),
			})
			if err != nil {
				log.WithError(err).Errorf("cannot place buy order")
				return
			}
		}
	})

	bbgo.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		_, _ = fmt.Fprintln(os.Stderr, s.TradeStats.String())
		wg.Done()
	})
	return nil
}
