package pivotshort

import (
	"context"
	"fmt"
	"os"
	"sort"
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

// BreakLow -- when price breaks the previous pivot low, we set a trade entry
type BreakLow struct {
	// Ratio is a number less than 1.0, price * ratio will be the price triggers the short order.
	Ratio fixedpoint.Value `json:"ratio"`

	// MarketOrder is the option to enable market order short.
	MarketOrder bool `json:"marketOrder"`

	// BounceRatio is a ratio used for placing the limit order sell price
	// limit sell price = breakLowPrice * (1 + BounceRatio)
	BounceRatio fixedpoint.Value `json:"bounceRatio"`

	Quantity     fixedpoint.Value      `json:"quantity"`
	StopEMARange fixedpoint.Value      `json:"stopEMARange"`
	StopEMA      *types.IntervalWindow `json:"stopEMA"`
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
	BreakLow BreakLow `json:"breakLow"`

	// ResistanceShort is one of the entry method
	ResistanceShort *ResistanceShort `json:"resistanceShort"`

	ExitMethods bbgo.ExitMethodSet `json:"exits"`

	session       *bbgo.ExchangeSession
	orderExecutor *bbgo.GeneralOrderExecutor

	lastLow         fixedpoint.Value
	pivot           *indicator.Pivot
	resistancePivot *indicator.Pivot
	stopEWMA        *indicator.EWMA
	pivotLowPrices  []fixedpoint.Value

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

	s.lastLow = fixedpoint.Zero

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

	store, _ := session.MarketDataStore(s.Symbol)
	standardIndicator, _ := session.StandardIndicatorSet(s.Symbol)

	s.pivot = &indicator.Pivot{IntervalWindow: s.IntervalWindow}
	s.pivot.Bind(store)
	preloadPivot(s.pivot, store)

	// update pivot low data
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		lastLow := fixedpoint.NewFromFloat(s.pivot.LastLow())
		if lastLow.IsZero() {
			return
		}

		if lastLow.Compare(s.lastLow) != 0 {
			log.Infof("new pivot low detected: %f %s", s.pivot.LastLow(), kline.EndTime.Time())
		}

		s.lastLow = lastLow
		s.pivotLowPrices = append(s.pivotLowPrices, s.lastLow)
	}))

	if s.BreakLow.StopEMA != nil {
		s.stopEWMA = standardIndicator.EWMA(*s.BreakLow.StopEMA)
	}

	for _, method := range s.ExitMethods {
		method.Bind(session, s.orderExecutor)
	}

	if s.ResistanceShort != nil && s.ResistanceShort.Enabled {
		s.ResistanceShort.Bind(session, s.orderExecutor)
	}

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1m, func(kline types.KLine) {
		if s.Status != types.StrategyStatusRunning {
			return
		}

		if s.Position.IsOpened(kline.Close) {
			return
		}

		if len(s.pivotLowPrices) == 0 {
			log.Infof("currently there is no pivot low prices, skip placing orders...")
			return
		}

		previousLow := s.pivotLowPrices[len(s.pivotLowPrices)-1]

		// truncate the pivot low prices
		if len(s.pivotLowPrices) > 10 {
			s.pivotLowPrices = s.pivotLowPrices[len(s.pivotLowPrices)-10:]
		}

		ratio := fixedpoint.One.Add(s.BreakLow.Ratio)
		breakPrice := previousLow.Mul(ratio)

		openPrice := kline.Open
		closePrice := kline.Close
		// if previous low is not break, skip
		if closePrice.Compare(breakPrice) >= 0 {
			return
		}

		// we need the price cross the break line or we do nothing
		if !(openPrice.Compare(breakPrice) > 0 && closePrice.Compare(breakPrice) < 0) {
			return
		}

		log.Infof("%s breakLow signal detected, closed price %f < breakPrice %f", kline.Symbol, closePrice.Float64(), breakPrice.Float64())

		// stop EMA protection
		if s.stopEWMA != nil {
			ema := fixedpoint.NewFromFloat(s.stopEWMA.Last())
			if ema.IsZero() {
				return
			}

			emaStopShortPrice := ema.Mul(fixedpoint.One.Sub(s.BreakLow.StopEMARange))
			if closePrice.Compare(emaStopShortPrice) < 0 {
				log.Infof("stopEMA protection: close price %f < EMA(%v) = %f", closePrice.Float64(), s.BreakLow.StopEMA, ema.Float64())
				return
			}
		}

		// graceful cancel all active orders
		_ = s.orderExecutor.GracefulCancel(ctx)

		quantity := s.useQuantityOrBaseBalance(s.BreakLow.Quantity)
		if s.BreakLow.MarketOrder {
			bbgo.Notify("%s price %f breaks the previous low %f with ratio %f, submitting market sell to open a short position", s.Symbol, kline.Close.Float64(), previousLow.Float64(), s.BreakLow.Ratio.Float64())
			s.placeMarketSell(ctx, quantity, "breakLowMarket")
		} else {
			sellPrice := previousLow.Mul(fixedpoint.One.Add(s.BreakLow.BounceRatio))

			bbgo.Notify("%s price %f breaks the previous low %f with ratio %f, submitting limit sell @ %f", s.Symbol, kline.Close.Float64(), previousLow.Float64(), s.BreakLow.Ratio.Float64(), sellPrice.Float64())
			s.placeLimitSell(ctx, sellPrice, quantity, "breakLowLimit")
		}
	}))

	if !bbgo.IsBackTesting {
		// use market trade to submit short order
		session.MarketDataStream.OnMarketTrade(func(trade types.Trade) {

		})
	}

	bbgo.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		_, _ = fmt.Fprintln(os.Stderr, s.TradeStats.String())
		wg.Done()
	})

	return nil
}

func (s *Strategy) findHigherPivotLow(price fixedpoint.Value) (fixedpoint.Value, bool) {
	for l := len(s.pivotLowPrices) - 1; l > 0; l-- {
		if s.pivotLowPrices[l].Compare(price) > 0 {
			return s.pivotLowPrices[l], true
		}
	}

	return price, false
}

func (s *Strategy) placeOrder(ctx context.Context, price fixedpoint.Value, quantity fixedpoint.Value) {
	_, _ = s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimitMaker,
		Price:    price,
		Quantity: quantity,
	})
}

func (s *Strategy) useQuantityOrBaseBalance(quantity fixedpoint.Value) fixedpoint.Value {
	if s.session.Margin || s.session.IsolatedMargin || s.session.Futures || s.session.IsolatedFutures {
		return quantity
	}

	balance, hasBalance := s.session.Account.Balance(s.Market.BaseCurrency)

	if hasBalance {
		if quantity.IsZero() {
			bbgo.Notify("sell quantity is not set, submitting sell with all base balance: %s", balance.Available.String())
			quantity = balance.Available
		} else {
			quantity = fixedpoint.Min(quantity, balance.Available)
		}
	}

	if quantity.IsZero() {
		log.Errorf("quantity is zero, can not submit sell order, please check settings")
	}

	return quantity
}

func (s *Strategy) placeLimitSell(ctx context.Context, price, quantity fixedpoint.Value, tag string) {
	_, _ = s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:           s.Symbol,
		Price:            price,
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeLimit,
		Quantity:         quantity,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
		Tag:              tag,
	})
}

func (s *Strategy) placeMarketSell(ctx context.Context, quantity fixedpoint.Value, tag string) {
	_, _ = s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:           s.Symbol,
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeMarket,
		Quantity:         quantity,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
		Tag:              tag,
	})
}

func findPossibleResistancePrices(closePrice float64, minDistance float64, lows []float64) []float64 {
	// sort float64 in increasing order
	// lower to higher prices
	sort.Float64s(lows)

	var resistancePrices []float64
	for _, low := range lows {
		if low < closePrice {
			continue
		}

		last := closePrice
		if len(resistancePrices) > 0 {
			last = resistancePrices[len(resistancePrices)-1]
		}

		if (low / last) < (1.0 + minDistance) {
			continue
		}
		resistancePrices = append(resistancePrices, low)
	}

	return resistancePrices
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
