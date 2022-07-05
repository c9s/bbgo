package supertrend

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/c9s/bbgo/pkg/util"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "supertrend"

const stateKey = "state-v1"

var log = logrus.WithField("strategy", ID)

// TODO: limit order for ATR TP

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

// LinGre is Linear Regression baseline
type LinGre struct {
	types.IntervalWindow
	baseLineSlope float64
}

// Update Linear Regression baseline slope
func (lg *LinGre) Update(klines []types.KLine) {
	if len(klines) < lg.Window {
		lg.baseLineSlope = 0
		return
	}

	var sumX, sumY, sumXSqr, sumXY float64 = 0, 0, 0, 0
	end := len(klines) - 1 // The last kline
	for i := end; i >= end-lg.Window+1; i-- {
		val := klines[i].GetClose().Float64()
		per := float64(end - i + 1)
		sumX += per
		sumY += val
		sumXSqr += per * per
		sumXY += val * per
	}
	length := float64(lg.Window)
	slope := (length*sumXY - sumX*sumY) / (length*sumXSqr - sumX*sumX)
	average := sumY / length
	endPrice := average - slope*sumX/length + slope
	startPrice := endPrice + slope*(length-1)
	lg.baseLineSlope = (length - 1) / (endPrice - startPrice)

	log.Debugf("linear regression baseline slope: %f", lg.baseLineSlope)
}

func (lg *LinGre) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if lg.Interval != interval {
		return
	}

	lg.Update(window)
}

func (lg *LinGre) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(lg.handleKLineWindowUpdate)
}

type Strategy struct {
	*bbgo.Graceful
	*bbgo.Persistence

	Environment *bbgo.Environment
	session     *bbgo.ExchangeSession
	Market      types.Market

	// persistence fields
	Position    *types.Position    `persistence:"position"`
	ProfitStats *types.ProfitStats `persistence:"profit_stats"`
	TradeStats  *types.TradeStats  `persistence:"trade_stats"`

	// Order and trade
	orderExecutor *bbgo.GeneralOrderExecutor

	// groupID is the group ID used for the strategy instance for canceling orders
	groupID uint32

	stopC chan struct{}

	// Symbol is the market symbol you want to trade
	Symbol string `json:"symbol"`

	// Interval is how long do you want to update your order price and quantity
	Interval types.Interval `json:"interval"`

	// FastDEMAWindow DEMA window for checking breakout
	FastDEMAWindow int `json:"fastDEMAWindow"`
	// SlowDEMAWindow DEMA window for checking breakout
	SlowDEMAWindow int `json:"slowDEMAWindow"`
	fastDEMA       *indicator.DEMA
	slowDEMA       *indicator.DEMA

	// SuperTrend indicator
	// SuperTrend SuperTrend `json:"superTrend"`
	Supertrend *indicator.Supertrend
	// SupertrendWindow ATR window for calculation of supertrend
	SupertrendWindow int `json:"supertrendWindow"`
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

	currentTakeProfitPrice fixedpoint.Value
	currentStopLossPrice   fixedpoint.Value

	// ExitMethods Exit methods
	ExitMethods []bbgo.ExitMethod `json:"exits"`

	// StrategyController
	bbgo.StrategyController
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

	if s.Leverage == 0.0 {
		return errors.New("leverage is required")
	}

	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
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

// setupIndicators initializes indicators
func (s *Strategy) setupIndicators() {
	// K-line store for indicators
	kLineStore, _ := s.session.MarketDataStore(s.Symbol)

	// DEMA
	if s.FastDEMAWindow == 0 {
		s.FastDEMAWindow = 144
	}
	s.fastDEMA = &indicator.DEMA{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.FastDEMAWindow}}

	if s.SlowDEMAWindow == 0 {
		s.SlowDEMAWindow = 169
	}
	s.slowDEMA = &indicator.DEMA{IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.SlowDEMAWindow}}
	// Preload
	if klines, ok := kLineStore.KLinesOfInterval(s.fastDEMA.Interval); ok {
		for i := 0; i < len(*klines); i++ {
			s.fastDEMA.Update((*klines)[i].GetClose().Float64())
		}
	}
	if klines, ok := kLineStore.KLinesOfInterval(s.slowDEMA.Interval); ok {
		for i := 0; i < len(*klines); i++ {
			s.slowDEMA.Update((*klines)[i].GetClose().Float64())
		}
	}

	// Supertrend
	if s.SupertrendWindow == 0 {
		s.SupertrendWindow = 39
	}
	if s.SupertrendMultiplier == 0 {
		s.SupertrendMultiplier = 3
	}
	s.Supertrend = &indicator.Supertrend{IntervalWindow: types.IntervalWindow{Window: s.SupertrendWindow, Interval: s.Interval}, ATRMultiplier: s.SupertrendMultiplier}
	s.Supertrend.AverageTrueRange = &indicator.ATR{IntervalWindow: types.IntervalWindow{Window: s.SupertrendWindow, Interval: s.Interval}}
	s.Supertrend.Bind(kLineStore)
	// Preload
	if klines, ok := kLineStore.KLinesOfInterval(s.Supertrend.Interval); ok {
		for i := 0; i < len(*klines); i++ {
			s.Supertrend.Update((*klines)[i].GetHigh().Float64(), (*klines)[i].GetLow().Float64(), (*klines)[i].GetClose().Float64())
		}
	}

	// Linear Regression
	if s.LinearRegression != nil {
		if s.LinearRegression.Window == 0 {
			s.LinearRegression = nil
		} else {
			s.LinearRegression.Bind(kLineStore)

			// Preload
			if klines, ok := kLineStore.KLinesOfInterval(s.LinearRegression.Interval); ok {
				s.LinearRegression.Update((*klines)[0:])
			}
		}
	}
}

func (s *Strategy) generateOrderForm(side types.SideType, quantity fixedpoint.Value, marginOrderSideEffect types.MarginOrderSideEffectType) types.SubmitOrder {
	orderForm := types.SubmitOrder{
		Symbol:           s.Symbol,
		Market:           s.Market,
		Side:             side,
		Type:             types.OrderTypeMarket,
		Quantity:         quantity,
		MarginSideEffect: marginOrderSideEffect,
		GroupID:          s.groupID,
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

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.session = session

	// calculate group id for orders
	instanceID := s.InstanceID()
	s.groupID = util.FNV32(instanceID)

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

	// Trade stats
	if s.TradeStats == nil {
		s.TradeStats = &types.TradeStats{}
	}

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

	// Sync position to redis on trade
	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(s)
	})

	s.stopC = make(chan struct{})

	// StrategyController
	s.Status = types.StrategyStatusRunning

	s.OnSuspend(func() {
		_ = s.orderExecutor.GracefulCancel(ctx)
		_ = s.Persistence.Sync(s)
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

	s.currentStopLossPrice = fixedpoint.Zero
	s.currentTakeProfitPrice = fixedpoint.Zero

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// StrategyController
		if s.Status != types.StrategyStatusRunning {
			return
		}

		// skip k-lines from other symbols or other intervals
		if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
			return
		}

		closePrice := kline.GetClose().Float64()
		openPrice := kline.GetOpen().Float64()

		// Supertrend signal
		stSignal := s.Supertrend.GetSignal()

		// DEMA signal
		var demaSignal types.Direction
		if closePrice > s.fastDEMA.Last() && closePrice > s.slowDEMA.Last() && !(openPrice > s.fastDEMA.Last() && openPrice > s.slowDEMA.Last()) {
			demaSignal = types.DirectionUp
		} else if closePrice < s.fastDEMA.Last() && closePrice < s.slowDEMA.Last() && !(openPrice < s.fastDEMA.Last() && openPrice < s.slowDEMA.Last()) {
			demaSignal = types.DirectionDown
		} else {
			demaSignal = types.DirectionNone
		}

		// Linear Regression signal
		var lgSignal types.Direction
		if s.LinearRegression != nil {
			switch {
			case s.LinearRegression.baseLineSlope > 0:
				lgSignal = types.DirectionUp
			case s.LinearRegression.baseLineSlope < 0:
				lgSignal = types.DirectionDown
			default:
				lgSignal = types.DirectionNone
			}
		}

		base := s.Position.GetBase()
		baseSign := base.Sign()

		// TP/SL if there's non-dust position and meets the criteria
		if !s.Market.IsDustQuantity(base.Abs(), kline.GetClose()) {
			stopNow := false

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

			if stopNow {
				if err := s.ClosePosition(ctx, fixedpoint.One); err == nil {
					s.currentStopLossPrice = fixedpoint.Zero
					s.currentTakeProfitPrice = fixedpoint.Zero
				}
			}
		}

		// Open position
		var side types.SideType
		if stSignal == types.DirectionUp && demaSignal == types.DirectionUp && (s.LinearRegression == nil || lgSignal == types.DirectionUp) {
			side = types.SideTypeBuy
			if s.StopLossByTriggeringK {
				s.currentStopLossPrice = kline.GetLow()
			}
			if s.TakeProfitAtrMultiplier > 0 {
				s.currentTakeProfitPrice = kline.GetClose().Add(fixedpoint.NewFromFloat(s.Supertrend.AverageTrueRange.Last() * s.TakeProfitAtrMultiplier))
			}
		} else if stSignal == types.DirectionDown && demaSignal == types.DirectionDown && (s.LinearRegression == nil || lgSignal == types.DirectionDown) {
			side = types.SideTypeSell
			if s.StopLossByTriggeringK {
				s.currentStopLossPrice = kline.GetHigh()
			}
			if s.TakeProfitAtrMultiplier > 0 {
				s.currentTakeProfitPrice = kline.GetClose().Sub(fixedpoint.NewFromFloat(s.Supertrend.AverageTrueRange.Last() * s.TakeProfitAtrMultiplier))
			}
		}

		// The default value of side is an empty string. Unless side is set by the checks above, the result of the following condition is false
		if side == types.SideTypeSell || side == types.SideTypeBuy {
			bbgo.Notify("open %s position for signal %v", s.Symbol, side)
			// Close opposite position if any
			if !s.Position.IsDust(kline.GetClose()) {
				if (side == types.SideTypeSell && s.Position.IsLong()) || (side == types.SideTypeBuy && s.Position.IsShort()) {
					bbgo.Notify("close existing %s position before open a new position", s.Symbol)
					_ = s.ClosePosition(ctx, fixedpoint.One)
				} else {
					bbgo.Notify("existing %s position has the same direction with the signal", s.Symbol)
					return
				}
			}

			orderForm := s.generateOrderForm(side, s.calculateQuantity(kline.GetClose()), types.SideEffectTypeMarginBuy)
			log.Infof("submit open position order %v", orderForm)
			_, err := s.orderExecutor.SubmitOrders(ctx, orderForm)
			if err != nil {
				log.WithError(err).Errorf("can not place %s open position order", s.Symbol)
				bbgo.Notify("can not place %s open position order", s.Symbol)
			}
		}
	})

	// Graceful shutdown
	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		close(s.stopC)

		_ = s.orderExecutor.GracefulCancel(ctx)
		_, _ = fmt.Fprintln(os.Stderr, s.TradeStats.String())
	})

	return nil
}
