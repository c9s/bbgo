package pivotshort

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/dynamic"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "pivotshort"

var one = fixedpoint.One
var zero = fixedpoint.Zero

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type IntervalWindowSetting struct {
	types.IntervalWindow
}

type SupportTakeProfit struct {
	Symbol string
	types.IntervalWindow
	Ratio fixedpoint.Value `json:"ratio"`

	pivot               *indicator.Pivot
	orderExecutor       *bbgo.GeneralOrderExecutor
	session             *bbgo.ExchangeSession
	activeOrders        *bbgo.ActiveOrderBook
	currentSupportPrice fixedpoint.Value
}

func (s *SupportTakeProfit) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *SupportTakeProfit) updateSupportPrice(closePrice fixedpoint.Value) bool {
	supportPrices := findPossibleSupportPrices(closePrice.Float64(), 0.05, s.pivot.Lows)
	if len(supportPrices) == 0 {
		return false
	}

	// nextSupportPrice are sorted in decreasing order
	nextSupportPrice := fixedpoint.NewFromFloat(supportPrices[0])
	currentBuyPrice := s.currentSupportPrice.Mul(one.Add(s.Ratio))

	if s.currentSupportPrice.IsZero() {
		log.Infof("setup next support take profit price at %f", nextSupportPrice.Float64())
		s.currentSupportPrice = nextSupportPrice
		return true
	}

	// the close price is already lower than the support price, than we should update
	if closePrice.Compare(currentBuyPrice) < 0 || nextSupportPrice.Compare(s.currentSupportPrice) > 0 {
		log.Infof("setup next support take profit price at %f", nextSupportPrice.Float64())
		s.currentSupportPrice = nextSupportPrice
		return true
	}

	return false
}

func (s *SupportTakeProfit) Bind(session *bbgo.ExchangeSession, orderExecutor *bbgo.GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor
	s.activeOrders = bbgo.NewActiveOrderBook(s.Symbol)

	position := orderExecutor.Position()
	symbol := position.Symbol
	store, _ := session.MarketDataStore(symbol)
	s.pivot = &indicator.Pivot{IntervalWindow: s.IntervalWindow}
	s.pivot.Bind(store)
	preloadPivot(s.pivot, store)

	session.UserDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		if !position.IsOpened(kline.Close) {
			return
		}

		if !s.updateSupportPrice(kline.Close) {
			return
		}

		buyPrice := s.currentSupportPrice.Mul(one.Add(s.Ratio))
		quantity := position.GetQuantity()
		ctx := context.Background()

		if err := orderExecutor.GracefulCancelActiveOrderBook(ctx, s.activeOrders); err != nil {
			log.WithError(err).Errorf("cancel order failed")
		}

		bbgo.Notify("placing %s take profit order at price %f", s.Symbol, buyPrice.Float64())
		createdOrders, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
			Symbol:   symbol,
			Type:     types.OrderTypeLimitMaker,
			Price:    buyPrice,
			Quantity: quantity,
			Tag:      "supportTakeProfit",
		})

		if err != nil {
			log.WithError(err).Errorf("can not submit orders: %+v", createdOrders)
		}

		s.activeOrders.Add(createdOrders...)
	}))
}

type Strategy struct {
	Environment *bbgo.Environment
	Symbol      string `json:"symbol"`
	Market      types.Market

	// pivot interval and window
	types.IntervalWindow

	// persistence fields
	Position    *types.Position    `persistence:"position"`
	ProfitStats *types.ProfitStats `persistence:"profit_stats"`
	TradeStats  *types.TradeStats  `persistence:"trade_stats"`

	// BreakLow is one of the entry method
	BreakLow *BreakLow `json:"breakLow"`

	// ResistanceShort is one of the entry method
	ResistanceShort *ResistanceShort `json:"resistanceShort"`

	SupportTakeProfit []SupportTakeProfit `json:"supportTakeProfit"`

	ExitMethods bbgo.ExitMethodSet `json:"exits"`

	session       *bbgo.ExchangeSession
	orderExecutor *bbgo.GeneralOrderExecutor

	// StrategyController
	bbgo.StrategyController
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})

	if s.ResistanceShort != nil && s.ResistanceShort.Enabled {
		dynamic.InheritStructValues(s.ResistanceShort, s)
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.ResistanceShort.Interval})
	}

	if s.BreakLow != nil {
		dynamic.InheritStructValues(s.BreakLow, s)
		s.BreakLow.Subscribe(session)
	}

	for i := range s.SupportTakeProfit {
		dynamic.InheritStructValues(&s.SupportTakeProfit[i], s)
		s.SupportTakeProfit[i].Subscribe(session)
	}

	if !bbgo.IsBackTesting {
		session.Subscribe(types.MarketTradeChannel, s.Symbol, types.SubscribeOptions{})
	}

	s.ExitMethods.SetAndSubscribe(session, s)
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) CurrentPosition() *types.Position {
	return s.Position
}

func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	return s.orderExecutor.ClosePosition(ctx, percentage)
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
		s.TradeStats = &types.TradeStats{}
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
		_ = s.ClosePosition(ctx, fixedpoint.One)
	})

	// initial required information
	s.session = session
	s.orderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.orderExecutor.BindEnvironment(s.Environment)
	s.orderExecutor.BindProfitStats(s.ProfitStats)
	s.orderExecutor.BindTradeStats(s.TradeStats)
	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(s)
	})
	s.orderExecutor.Bind()

	for _, method := range s.ExitMethods {
		method.Bind(session, s.orderExecutor)
	}

	if s.ResistanceShort != nil && s.ResistanceShort.Enabled {
		s.ResistanceShort.Bind(session, s.orderExecutor)
	}

	if s.BreakLow != nil {
		s.BreakLow.Bind(session, s.orderExecutor)
	}

	for _, m := range s.SupportTakeProfit {
		m.Bind(session, s.orderExecutor)
	}

	bbgo.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		_, _ = fmt.Fprintln(os.Stderr, s.TradeStats.String())
		_ = s.orderExecutor.GracefulCancel(ctx)
	})

	return nil
}

func preloadPivot(pivot *indicator.Pivot, store *bbgo.MarketDataStore) *types.KLine {
	klines, ok := store.KLinesOfInterval(pivot.Interval)
	if !ok {
		return nil
	}

	last := (*klines)[len(*klines)-1]
	log.Debugf("updating pivot indicator: %d klines", len(*klines))

	for i := pivot.Window; i < len(*klines); i++ {
		pivot.Update((*klines)[0 : i+1])
	}

	log.Debugf("found %v previous lows: %v", pivot.IntervalWindow, pivot.Lows)
	log.Debugf("found %v previous highs: %v", pivot.IntervalWindow, pivot.Highs)
	return &last
}
