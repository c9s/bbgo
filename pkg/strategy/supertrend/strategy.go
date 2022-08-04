package supertrend

import (
	"bufio"
	"context"
	"fmt"
	"github.com/fatih/color"
	"os"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "supertrend"

var log = logrus.WithField("strategy", ID)

// TODO: limit order for ATR TP
func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Environment *bbgo.Environment
	Market      types.Market

	// persistence fields
	Position    *types.Position    `persistence:"position"`
	ProfitStats *types.ProfitStats `persistence:"profit_stats"`
	TradeStats  *types.TradeStats  `persistence:"trade_stats"`

	// Symbol is the market symbol you want to trade
	Symbol string `json:"symbol"`

	types.IntervalWindow

	// Double DEMA
	doubleDema *DoubleDema
	// FastDEMAWindow DEMA window for checking breakout
	FastDEMAWindow int `json:"fastDEMAWindow"`
	// SlowDEMAWindow DEMA window for checking breakout
	SlowDEMAWindow int `json:"slowDEMAWindow"`

	// SuperTrend indicator
	Supertrend *indicator.Supertrend
	// SupertrendMultiplier ATR multiplier for calculation of supertrend
	SupertrendMultiplier float64 `json:"supertrendMultiplier"`

	// LinearRegression Use linear regression as trend confirmation
	LinearRegression *LinGre `json:"linearRegression,omitempty"`

	// Leverage
	Leverage float64 `json:"leverage"`

	// TakeProfitAtrMultiplier TP according to ATR multiple, 0 to disable this
	TakeProfitAtrMultiplier float64 `json:"takeProfitAtrMultiplier"`

	// StopLossByTriggeringK Set SL price to the low/high of the triggering Kline
	StopLossByTriggeringK bool `json:"stopLossByTriggeringK"`

	// StopByReversedSupertrend TP/SL by reversed supertrend signal
	StopByReversedSupertrend bool `json:"stopByReversedSupertrend"`

	// StopByReversedDema TP/SL by reversed DEMA signal
	StopByReversedDema bool `json:"stopByReversedDema"`

	// StopByReversedLinGre TP/SL by reversed linear regression signal
	StopByReversedLinGre bool `json:"stopByReversedLinGre"`

	// ExitMethods Exit methods
	ExitMethods bbgo.ExitMethodSet `json:"exits"`

	session                *bbgo.ExchangeSession
	orderExecutor          *bbgo.GeneralOrderExecutor
	currentTakeProfitPrice fixedpoint.Value
	currentStopLossPrice   fixedpoint.Value

	// StrategyController
	bbgo.StrategyController

	// Accumulated profit report
	accumulatedProfit   fixedpoint.Value
	accumulatedProfitMA *indicator.SMA
	// AccumulatedProfitMAWindow Accumulated profit SMA window
	AccumulatedProfitMAWindow int `json:"accumulatedProfitMAWindow"`
	dailyAccumulatedProfits   types.Float64Slice
	lastDayAccumulatedProfit  fixedpoint.Value
	// AccumulatedProfitLastPeriodWindow Last period window of accumulated profit
	AccumulatedProfitLastPeriodWindow int `json:"accumulatedProfitLastPeriodWindow"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Validate() error {
	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	if len(s.Interval) == 0 {
		return errors.New("interval is required")
	}

	if s.Leverage <= 0.0 {
		return errors.New("leverage is required")
	}

	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.LinearRegression.Interval})

	s.ExitMethods.SetAndSubscribe(session, s)

	// Accumulated profit report
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1d})
}

// Position control

func (s *Strategy) CurrentPosition() *types.Position {
	return s.Position
}

func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	base := s.Position.GetBase()
	if base.IsZero() {
		return fmt.Errorf("no opened %s position", s.Position.Symbol)
	}

	// make it negative
	quantity := base.Mul(percentage).Abs()
	side := types.SideTypeBuy
	if base.Sign() > 0 {
		side = types.SideTypeSell
	}

	if quantity.Compare(s.Market.MinQuantity) < 0 {
		return fmt.Errorf("%s order quantity %v is too small, less than %v", s.Symbol, quantity, s.Market.MinQuantity)
	}

	orderForm := s.generateOrderForm(side, quantity, types.SideEffectTypeAutoRepay)

	bbgo.Notify("submitting %s %s order to close position by %v", s.Symbol, side.String(), percentage, orderForm)

	_, err := s.orderExecutor.SubmitOrders(ctx, orderForm)
	if err != nil {
		log.WithError(err).Errorf("can not place %s position close order", s.Symbol)
		bbgo.Notify("can not place %s position close order", s.Symbol)
	}

	return err
}

// preloadSupertrend preloads supertrend indicator
func preloadSupertrend(supertrend *indicator.Supertrend, kLineStore *bbgo.MarketDataStore) {
	if klines, ok := kLineStore.KLinesOfInterval(supertrend.Interval); ok {
		for i := 0; i < len(*klines); i++ {
			supertrend.Update((*klines)[i].GetHigh().Float64(), (*klines)[i].GetLow().Float64(), (*klines)[i].GetClose().Float64())
		}
	}
}

// setupIndicators initializes indicators
func (s *Strategy) setupIndicators() {
	// K-line store for indicators
	kLineStore, _ := s.session.MarketDataStore(s.Symbol)

	// Double DEMA
	s.doubleDema = newDoubleDema(kLineStore, s.Interval, s.FastDEMAWindow, s.SlowDEMAWindow)

	// Supertrend
	if s.Window == 0 {
		s.Window = 39
	}
	if s.SupertrendMultiplier == 0 {
		s.SupertrendMultiplier = 3
	}
	s.Supertrend = &indicator.Supertrend{IntervalWindow: types.IntervalWindow{Window: s.Window, Interval: s.Interval}, ATRMultiplier: s.SupertrendMultiplier}
	s.Supertrend.AverageTrueRange = &indicator.ATR{IntervalWindow: types.IntervalWindow{Window: s.Window, Interval: s.Interval}}
	s.Supertrend.Bind(kLineStore)
	preloadSupertrend(s.Supertrend, kLineStore)

	// Linear Regression
	if s.LinearRegression != nil {
		if s.LinearRegression.Window == 0 {
			s.LinearRegression = nil
		} else if s.LinearRegression.Interval == "" {
			s.LinearRegression = nil
		} else {
			s.LinearRegression.Bind(kLineStore)
			s.LinearRegression.preload(kLineStore)
		}
	}
}

func (s *Strategy) shouldStop(kline types.KLine, stSignal types.Direction, demaSignal types.Direction, lgSignal types.Direction) bool {
	stopNow := false
	base := s.Position.GetBase()
	baseSign := base.Sign()

	if s.StopLossByTriggeringK && !s.currentStopLossPrice.IsZero() && ((baseSign < 0 && kline.GetClose().Compare(s.currentStopLossPrice) > 0) || (baseSign > 0 && kline.GetClose().Compare(s.currentStopLossPrice) < 0)) {
		// SL by triggering Kline low/high
		bbgo.Notify("%s stop loss by triggering the kline low/high", s.Symbol)
		stopNow = true
	} else if s.TakeProfitAtrMultiplier > 0 && !s.currentTakeProfitPrice.IsZero() && ((baseSign < 0 && kline.GetClose().Compare(s.currentTakeProfitPrice) < 0) || (baseSign > 0 && kline.GetClose().Compare(s.currentTakeProfitPrice) > 0)) {
		// TP by multiple of ATR
		bbgo.Notify("%s take profit by multiple of ATR", s.Symbol)
		stopNow = true
	} else if s.StopByReversedSupertrend && ((baseSign < 0 && stSignal == types.DirectionUp) || (baseSign > 0 && stSignal == types.DirectionDown)) {
		// Use supertrend signal to TP/SL
		bbgo.Notify("%s stop by the reversed signal of Supertrend", s.Symbol)
		stopNow = true
	} else if s.StopByReversedDema && ((baseSign < 0 && demaSignal == types.DirectionUp) || (baseSign > 0 && demaSignal == types.DirectionDown)) {
		// Use DEMA signal to TP/SL
		bbgo.Notify("%s stop by the reversed signal of DEMA", s.Symbol)
		stopNow = true
	} else if s.StopByReversedLinGre && ((baseSign < 0 && lgSignal == types.DirectionUp) || (baseSign > 0 && lgSignal == types.DirectionDown)) {
		// Use linear regression signal to TP/SL
		bbgo.Notify("%s stop by the reversed signal of linear regression", s.Symbol)
		stopNow = true
	}

	return stopNow
}

func (s *Strategy) getSide(stSignal types.Direction, demaSignal types.Direction, lgSignal types.Direction) types.SideType {
	var side types.SideType

	if stSignal == types.DirectionUp && demaSignal == types.DirectionUp && (s.LinearRegression == nil || lgSignal == types.DirectionUp) {
		side = types.SideTypeBuy
	} else if stSignal == types.DirectionDown && demaSignal == types.DirectionDown && (s.LinearRegression == nil || lgSignal == types.DirectionDown) {
		side = types.SideTypeSell
	}

	return side
}

func (s *Strategy) generateOrderForm(side types.SideType, quantity fixedpoint.Value, marginOrderSideEffect types.MarginOrderSideEffectType) types.SubmitOrder {
	orderForm := types.SubmitOrder{
		Symbol:           s.Symbol,
		Market:           s.Market,
		Side:             side,
		Type:             types.OrderTypeMarket,
		Quantity:         quantity,
		MarginSideEffect: marginOrderSideEffect,
	}

	return orderForm
}

// calculateQuantity returns leveraged quantity
func (s *Strategy) calculateQuantity(currentPrice fixedpoint.Value) fixedpoint.Value {
	balance, ok := s.session.GetAccount().Balance(s.Market.QuoteCurrency)
	if !ok {
		log.Errorf("can not update %s balance from exchange", s.Symbol)
		return fixedpoint.Zero
	}

	amountAvailable := balance.Available.Mul(fixedpoint.NewFromFloat(s.Leverage))
	quantity := amountAvailable.Div(currentPrice)

	return quantity
}

// PrintResult prints accumulated profit status
func (s *Strategy) PrintResult(o *os.File) {
	f := bufio.NewWriter(o)
	defer f.Flush()
	hiyellow := color.New(color.FgHiYellow).FprintfFunc()
	hiyellow(f, "------ %s Accumulated Profit Results ------\n", s.InstanceID())
	hiyellow(f, "Symbol: %v\n", s.Symbol)
	hiyellow(f, "Accumulated Profit: %v\n", s.accumulatedProfit)
	hiyellow(f, "Accumulated Profit %dMA: %f\n", s.AccumulatedProfitMAWindow, s.accumulatedProfitMA.Last())
	hiyellow(f, "Last %d day(s) Accumulated Profit: %f\n", s.AccumulatedProfitLastPeriodWindow, s.dailyAccumulatedProfits.Sum())
	hiyellow(f, "\n")
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.session = session

	s.currentStopLossPrice = fixedpoint.Zero
	s.currentTakeProfitPrice = fixedpoint.Zero

	// calculate group id for orders
	instanceID := s.InstanceID()

	// If position is nil, we need to allocate a new position for calculation
	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}
	// Always update the position fields
	s.Position.Strategy = ID
	s.Position.StrategyInstanceID = s.InstanceID()

	// Profit stats
	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	if s.TradeStats == nil {
		s.TradeStats = types.NewTradeStats(s.Symbol)
	}
	startTime := s.Environment.StartTime()
	s.TradeStats.SetIntervalProfitCollector(types.NewIntervalProfitCollector(types.Interval1d, startTime))
	s.TradeStats.SetIntervalProfitCollector(types.NewIntervalProfitCollector(types.Interval1w, startTime))
	s.TradeStats.SetIntervalProfitCollector(types.NewIntervalProfitCollector(types.Interval1mo, startTime))

	// Set fee rate
	if s.session.MakerFeeRate.Sign() > 0 || s.session.TakerFeeRate.Sign() > 0 {
		s.Position.SetExchangeFeeRate(s.session.ExchangeName, types.ExchangeFee{
			MakerFeeRate: s.session.MakerFeeRate,
			TakerFeeRate: s.session.TakerFeeRate,
		})
	}

	// Setup order executor
	s.orderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.orderExecutor.BindEnvironment(s.Environment)
	s.orderExecutor.BindProfitStats(s.ProfitStats)
	s.orderExecutor.BindTradeStats(s.TradeStats)
	s.orderExecutor.Bind()

	// Accumulated profit report
	if s.AccumulatedProfitMAWindow <= 0 {
		s.AccumulatedProfitMAWindow = 60
	}
	s.accumulatedProfitMA = &indicator.SMA{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.AccumulatedProfitMAWindow}}
	s.orderExecutor.TradeCollector().OnProfit(func(trade types.Trade, profit *types.Profit) {
		if profit == nil {
			return
		}

		s.accumulatedProfit = s.accumulatedProfit.Add(profit.Profit)
		s.accumulatedProfitMA.Update(s.accumulatedProfit.Float64())
	})
	if s.AccumulatedProfitLastPeriodWindow <= 0 {
		s.AccumulatedProfitLastPeriodWindow = 7
	}
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1d, func(kline types.KLine) {
		s.dailyAccumulatedProfits.Update(s.accumulatedProfit.Sub(s.lastDayAccumulatedProfit).Float64())
		s.dailyAccumulatedProfits = s.dailyAccumulatedProfits.Tail(s.AccumulatedProfitLastPeriodWindow)
		s.lastDayAccumulatedProfit = s.accumulatedProfit
	}))

	// Sync position to redis on trade
	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(s)
	})

	// StrategyController
	s.Status = types.StrategyStatusRunning
	s.OnSuspend(func() {
		_ = s.orderExecutor.GracefulCancel(ctx)
		bbgo.Sync(s)
	})
	s.OnEmergencyStop(func() {
		_ = s.orderExecutor.GracefulCancel(ctx)
		// Close 100% position
		_ = s.ClosePosition(ctx, fixedpoint.One)
	})

	// Setup indicators
	s.setupIndicators()

	// Exit methods
	for _, method := range s.ExitMethods {
		method.Bind(session, s.orderExecutor)
	}

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		// StrategyController
		if s.Status != types.StrategyStatusRunning {
			return
		}

		closePrice := kline.GetClose()
		openPrice := kline.GetOpen()
		closePrice64 := closePrice.Float64()
		openPrice64 := openPrice.Float64()

		// Supertrend signal
		stSignal := s.Supertrend.GetSignal()

		// DEMA signal
		demaSignal := s.doubleDema.getDemaSignal(openPrice64, closePrice64)

		// Linear Regression signal
		var lgSignal types.Direction
		if s.LinearRegression != nil {
			lgSignal = s.LinearRegression.GetSignal()
		}

		// TP/SL if there's non-dust position and meets the criteria
		if !s.Market.IsDustQuantity(s.Position.GetBase().Abs(), closePrice) && s.shouldStop(kline, stSignal, demaSignal, lgSignal) {
			if err := s.ClosePosition(ctx, fixedpoint.One); err == nil {
				s.currentStopLossPrice = fixedpoint.Zero
				s.currentTakeProfitPrice = fixedpoint.Zero
			}
		}

		// Get order side
		side := s.getSide(stSignal, demaSignal, lgSignal)
		// Set TP/SL price if needed
		if side == types.SideTypeBuy {
			if s.StopLossByTriggeringK {
				s.currentStopLossPrice = kline.GetLow()
			}
			if s.TakeProfitAtrMultiplier > 0 {
				s.currentTakeProfitPrice = closePrice.Add(fixedpoint.NewFromFloat(s.Supertrend.AverageTrueRange.Last() * s.TakeProfitAtrMultiplier))
			}
		} else if side == types.SideTypeSell {
			if s.StopLossByTriggeringK {
				s.currentStopLossPrice = kline.GetHigh()
			}
			if s.TakeProfitAtrMultiplier > 0 {
				s.currentTakeProfitPrice = closePrice.Sub(fixedpoint.NewFromFloat(s.Supertrend.AverageTrueRange.Last() * s.TakeProfitAtrMultiplier))
			}
		}

		// Open position
		// The default value of side is an empty string. Unless side is set by the checks above, the result of the following condition is false
		if side == types.SideTypeSell || side == types.SideTypeBuy {
			bbgo.Notify("open %s position for signal %v", s.Symbol, side)
			// Close opposite position if any
			if !s.Position.IsDust(closePrice) {
				if (side == types.SideTypeSell && s.Position.IsLong()) || (side == types.SideTypeBuy && s.Position.IsShort()) {
					bbgo.Notify("close existing %s position before open a new position", s.Symbol)
					_ = s.ClosePosition(ctx, fixedpoint.One)
				} else {
					bbgo.Notify("existing %s position has the same direction with the signal", s.Symbol)
					return
				}
			}

			orderForm := s.generateOrderForm(side, s.calculateQuantity(closePrice), types.SideEffectTypeMarginBuy)
			log.Infof("submit open position order %v", orderForm)
			_, err := s.orderExecutor.SubmitOrders(ctx, orderForm)
			if err != nil {
				log.WithError(err).Errorf("can not place %s open position order", s.Symbol)
				bbgo.Notify("can not place %s open position order", s.Symbol)
			}
		}
	}))

	// Graceful shutdown
	bbgo.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		// Print accumulated profit report
		defer s.PrintResult(os.Stdout)

		_ = s.orderExecutor.GracefulCancel(ctx)
		_, _ = fmt.Fprintln(os.Stderr, s.TradeStats.String())
	})

	return nil
}
