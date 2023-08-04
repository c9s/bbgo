package supertrend

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/datatype/floats"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/report"
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

	// ProfitStatsTracker tracks profit related status and generates report
	ProfitStatsTracker *report.ProfitStatsTracker `json:"profitStatsTracker"`
	TrackParameters    bool                       `json:"trackParameters"`

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
	LinearRegression *LinReg `json:"linearRegression,omitempty"`

	// Leverage uses the account net value to calculate the order qty
	Leverage fixedpoint.Value `json:"leverage"`
	// Quantity sets the fixed order qty, takes precedence over Leverage
	Quantity               fixedpoint.Value `json:"quantity"`
	AccountValueCalculator *bbgo.AccountValueCalculator

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

	// whether to draw graph or not by the end of backtest
	DrawGraph       bool   `json:"drawGraph"`
	GraphPNLPath    string `json:"graphPNLPath"`
	GraphCumPNLPath string `json:"graphCumPNLPath"`

	// for position
	buyPrice     float64 `persistence:"buy_price"`
	sellPrice    float64 `persistence:"sell_price"`
	highestPrice float64 `persistence:"highest_price"`
	lowestPrice  float64 `persistence:"lowest_price"`

	session                *bbgo.ExchangeSession
	orderExecutor          *bbgo.GeneralOrderExecutor
	currentTakeProfitPrice fixedpoint.Value
	currentStopLossPrice   fixedpoint.Value

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

	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.LinearRegression.Interval})

	s.ExitMethods.SetAndSubscribe(session, s)

	// Profit tracker
	if s.ProfitStatsTracker != nil {
		s.ProfitStatsTracker.Subscribe(session, s.Symbol)
	}
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
	s.Supertrend.BindK(s.session.MarketDataStream, s.Symbol, s.Supertrend.Interval)
	if klines, ok := kLineStore.KLinesOfInterval(s.Supertrend.Interval); ok {
		s.Supertrend.LoadK((*klines)[0:])
	}

	// Linear Regression
	if s.LinearRegression != nil {
		if s.LinearRegression.Window == 0 {
			s.LinearRegression = nil
		} else if s.LinearRegression.Interval == "" {
			s.LinearRegression = nil
		} else {
			s.LinearRegression.BindK(s.session.MarketDataStream, s.Symbol, s.LinearRegression.Interval)
			if klines, ok := kLineStore.KLinesOfInterval(s.LinearRegression.Interval); ok {
				s.LinearRegression.LoadK((*klines)[0:])
			}
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
func (s *Strategy) calculateQuantity(ctx context.Context, currentPrice fixedpoint.Value, side types.SideType) fixedpoint.Value {
	// Quantity takes precedence
	if !s.Quantity.IsZero() {
		return s.Quantity
	}

	usingLeverage := s.session.Margin || s.session.IsolatedMargin || s.session.Futures || s.session.IsolatedFutures

	if bbgo.IsBackTesting { // Backtesting
		balance, ok := s.session.GetAccount().Balance(s.Market.QuoteCurrency)
		if !ok {
			log.Errorf("can not update %s quote balance from exchange", s.Symbol)
			return fixedpoint.Zero
		}

		return balance.Available.Mul(fixedpoint.Min(s.Leverage, fixedpoint.One)).Div(currentPrice)
	} else if !usingLeverage && side == types.SideTypeSell { // Spot sell
		balance, ok := s.session.GetAccount().Balance(s.Market.BaseCurrency)
		if !ok {
			log.Errorf("can not update %s base balance from exchange", s.Symbol)
			return fixedpoint.Zero
		}

		return balance.Available.Mul(fixedpoint.Min(s.Leverage, fixedpoint.One))
	} else { // Using leverage or spot buy
		quoteQty, err := bbgo.CalculateQuoteQuantity(ctx, s.session, s.Market.QuoteCurrency, s.Leverage)
		if err != nil {
			log.WithError(err).Errorf("can not update %s quote balance from exchange", s.Symbol)
			return fixedpoint.Zero
		}

		return quoteQty.Div(currentPrice)
	}
}

func (s *Strategy) CalcAssetValue(price fixedpoint.Value) fixedpoint.Value {
	balances := s.session.GetAccount().Balances()
	return balances[s.Market.BaseCurrency].Total().Mul(price).Add(balances[s.Market.QuoteCurrency].Total())
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

	// Interval profit report
	if bbgo.IsBackTesting {
		startTime := s.Environment.StartTime()
		s.TradeStats.SetIntervalProfitCollector(types.NewIntervalProfitCollector(types.Interval1d, startTime))
		s.TradeStats.SetIntervalProfitCollector(types.NewIntervalProfitCollector(types.Interval1w, startTime))
		s.TradeStats.SetIntervalProfitCollector(types.NewIntervalProfitCollector(types.Interval1mo, startTime))
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

	// Setup profit tracker
	if s.ProfitStatsTracker != nil {
		if s.ProfitStatsTracker.CurrentProfitStats == nil {
			s.ProfitStatsTracker.InitLegacy(s.Market, &s.ProfitStats, s.TradeStats)
		}

		// Add strategy parameters to report
		if s.TrackParameters && s.ProfitStatsTracker.AccumulatedProfitReport != nil {
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("window", strconv.Itoa(s.Window))
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("multiplier", strconv.FormatFloat(s.SupertrendMultiplier, 'f', 2, 64))
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("fastDEMA", strconv.Itoa(s.FastDEMAWindow))
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("slowDEMA", strconv.Itoa(s.SlowDEMAWindow))
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("linReg", strconv.Itoa(s.LinearRegression.Window))
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("takeProfitAtrMultiplier", strconv.FormatFloat(s.TakeProfitAtrMultiplier, 'f', 2, 64))
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("stopLossByTriggeringK", strconv.FormatBool(s.StopLossByTriggeringK))
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("stopByReversedSupertrend", strconv.FormatBool(s.StopByReversedSupertrend))
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("stopByReversedDema", strconv.FormatBool(s.StopByReversedDema))
			s.ProfitStatsTracker.AccumulatedProfitReport.AddStrategyParameter("stopByReversedLinGre", strconv.FormatBool(s.StopByReversedLinGre))
		}

		s.ProfitStatsTracker.Bind(s.session, s.orderExecutor.TradeCollector())
	}

	// AccountValueCalculator
	s.AccountValueCalculator = bbgo.NewAccountValueCalculator(s.session, s.Market.QuoteCurrency)

	// For drawing
	profitSlice := floats.Slice{1., 1.}
	price, _ := session.LastPrice(s.Symbol)
	initAsset := s.CalcAssetValue(price).Float64()
	cumProfitSlice := floats.Slice{initAsset, initAsset}

	s.orderExecutor.TradeCollector().OnTrade(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
		// For drawing/charting
		price := trade.Price.Float64()
		if s.buyPrice > 0 {
			profitSlice.Update(price / s.buyPrice)
			cumProfitSlice.Update(s.CalcAssetValue(trade.Price).Float64())
		} else if s.sellPrice > 0 {
			profitSlice.Update(s.sellPrice / price)
			cumProfitSlice.Update(s.CalcAssetValue(trade.Price).Float64())
		}
		if s.Position.IsDust(trade.Price) {
			s.buyPrice = 0
			s.sellPrice = 0
			s.highestPrice = 0
			s.lowestPrice = 0
		} else if s.Position.IsLong() {
			s.buyPrice = price
			s.sellPrice = 0
			s.highestPrice = s.buyPrice
			s.lowestPrice = 0
		} else {
			s.sellPrice = price
			s.buyPrice = 0
			s.highestPrice = 0
			s.lowestPrice = s.sellPrice
		}
	})

	s.InitDrawCommands(&profitSlice, &cumProfitSlice)

	// Sync position to redis on trade
	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})

	// StrategyController
	s.Status = types.StrategyStatusRunning
	s.OnSuspend(func() {
		_ = s.orderExecutor.GracefulCancel(ctx)
		bbgo.Sync(ctx, s)
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
				s.currentTakeProfitPrice = closePrice.Add(fixedpoint.NewFromFloat(s.Supertrend.AverageTrueRange.Last(0) * s.TakeProfitAtrMultiplier))
			}
		} else if side == types.SideTypeSell {
			if s.StopLossByTriggeringK {
				s.currentStopLossPrice = kline.GetHigh()
			}
			if s.TakeProfitAtrMultiplier > 0 {
				s.currentTakeProfitPrice = closePrice.Sub(fixedpoint.NewFromFloat(s.Supertrend.AverageTrueRange.Last(0) * s.TakeProfitAtrMultiplier))
			}
		}

		// Open position
		// The default value of side is an empty string. Unless side is set by the checks above, the result of the following condition is false
		if side == types.SideTypeSell || side == types.SideTypeBuy {
			bbgo.Notify("open %s position for signal %v", s.Symbol, side)

			amount := s.calculateQuantity(ctx, closePrice, side)

			// Add opposite position amount if any
			if (side == types.SideTypeSell && s.Position.IsLong()) || (side == types.SideTypeBuy && s.Position.IsShort()) {
				if bbgo.IsBackTesting {
					_ = s.ClosePosition(ctx, fixedpoint.One)
					bbgo.Notify("close existing %s position before open a new position", s.Symbol)
					amount = s.calculateQuantity(ctx, closePrice, side)
				} else {
					bbgo.Notify("add existing opposite position amount %f of %s to the amount %f of open new position order", s.Position.GetQuantity().Float64(), s.Symbol, amount.Float64())
					amount = amount.Add(s.Position.GetQuantity())
				}
			} else if !s.Position.IsDust(closePrice) {
				bbgo.Notify("existing %s position has the same direction as the signal", s.Symbol)
				return
			}

			orderForm := s.generateOrderForm(side, amount, types.SideEffectTypeMarginBuy)
			log.Infof("submit open position order %v", orderForm)
			_, err := s.orderExecutor.SubmitOrders(ctx, orderForm)
			if err != nil {
				log.WithError(err).Errorf("can not place %s open position order", s.Symbol)
				bbgo.Notify("can not place %s open position order", s.Symbol)
			}
		}
	}))

	// Graceful shutdown
	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		// Output profit report
		if s.ProfitStatsTracker != nil {
			if s.ProfitStatsTracker.AccumulatedProfitReport != nil {
				s.ProfitStatsTracker.AccumulatedProfitReport.Output()
			}
		}

		if bbgo.IsBackTesting {
			// Draw graph
			if s.DrawGraph {
				if err := s.Draw(&profitSlice, &cumProfitSlice); err != nil {
					log.WithError(err).Errorf("cannot draw graph")
				}
			}
		}

		_ = s.orderExecutor.GracefulCancel(ctx)
		_, _ = fmt.Fprintln(os.Stderr, s.TradeStats.String())
	})

	return nil
}
