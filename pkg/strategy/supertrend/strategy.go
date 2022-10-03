package supertrend

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/data/tsv"
	"github.com/c9s/bbgo/pkg/datatype/floats"

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

// AccumulatedProfitReport For accumulated profit report output
type AccumulatedProfitReport struct {
	// AccumulatedProfitMAWindow Accumulated profit SMA window, in number of trades
	AccumulatedProfitMAWindow int `json:"accumulatedProfitMAWindow"`

	// IntervalWindow interval window, in days
	IntervalWindow int `json:"intervalWindow"`

	// NumberOfInterval How many intervals to output to TSV
	NumberOfInterval int `json:"NumberOfInterval"`

	// TsvReportPath The path to output report to
	TsvReportPath string `json:"tsvReportPath"`

	// AccumulatedDailyProfitWindow The window to sum up the daily profit, in days
	AccumulatedDailyProfitWindow int `json:"accumulatedDailyProfitWindow"`

	// Accumulated profit
	accumulatedProfit         fixedpoint.Value
	accumulatedProfitPerDay   floats.Slice
	previousAccumulatedProfit fixedpoint.Value

	// Accumulated profit MA
	accumulatedProfitMA       *indicator.SMA
	accumulatedProfitMAPerDay floats.Slice

	// Daily profit
	dailyProfit floats.Slice

	// Accumulated fee
	accumulatedFee       fixedpoint.Value
	accumulatedFeePerDay floats.Slice

	// Win ratio
	winRatioPerDay floats.Slice

	// Profit factor
	profitFactorPerDay floats.Slice

	// Trade number
	dailyTrades               floats.Slice
	accumulatedTrades         int
	previousAccumulatedTrades int
}

func (r *AccumulatedProfitReport) Initialize() {
	if r.AccumulatedProfitMAWindow <= 0 {
		r.AccumulatedProfitMAWindow = 60
	}
	if r.IntervalWindow <= 0 {
		r.IntervalWindow = 7
	}
	if r.AccumulatedDailyProfitWindow <= 0 {
		r.AccumulatedDailyProfitWindow = 7
	}
	if r.NumberOfInterval <= 0 {
		r.NumberOfInterval = 1
	}
	r.accumulatedProfitMA = &indicator.SMA{IntervalWindow: types.IntervalWindow{Interval: types.Interval1d, Window: r.AccumulatedProfitMAWindow}}
}

func (r *AccumulatedProfitReport) RecordProfit(profit fixedpoint.Value) {
	r.accumulatedProfit = r.accumulatedProfit.Add(profit)
}

func (r *AccumulatedProfitReport) RecordTrade(fee fixedpoint.Value) {
	r.accumulatedFee = r.accumulatedFee.Add(fee)
	r.accumulatedTrades += 1
}

func (r *AccumulatedProfitReport) DailyUpdate(tradeStats *types.TradeStats) {
	// Daily profit
	r.dailyProfit.Update(r.accumulatedProfit.Sub(r.previousAccumulatedProfit).Float64())
	r.previousAccumulatedProfit = r.accumulatedProfit

	// Accumulated profit
	r.accumulatedProfitPerDay.Update(r.accumulatedProfit.Float64())

	// Accumulated profit MA
	r.accumulatedProfitMA.Update(r.accumulatedProfit.Float64())
	r.accumulatedProfitMAPerDay.Update(r.accumulatedProfitMA.Last())

	// Accumulated Fee
	r.accumulatedFeePerDay.Update(r.accumulatedFee.Float64())

	// Win ratio
	r.winRatioPerDay.Update(tradeStats.WinningRatio.Float64())

	// Profit factor
	r.profitFactorPerDay.Update(tradeStats.ProfitFactor.Float64())

	// Daily trades
	r.dailyTrades.Update(float64(r.accumulatedTrades - r.previousAccumulatedTrades))
	r.previousAccumulatedTrades = r.accumulatedTrades
}

// Output Accumulated profit report to a TSV file
func (r *AccumulatedProfitReport) Output(symbol string) {
	if r.TsvReportPath != "" {
		tsvwiter, err := tsv.AppendWriterFile(r.TsvReportPath)
		if err != nil {
			panic(err)
		}
		defer tsvwiter.Close()
		// Output symbol, total acc. profit, acc. profit 60MA, interval acc. profit, fee, win rate, profit factor
		_ = tsvwiter.Write([]string{"#", "Symbol", "accumulatedProfit", "accumulatedProfitMA", fmt.Sprintf("%dd profit", r.AccumulatedDailyProfitWindow), "accumulatedFee", "winRatio", "profitFactor", "60D trades"})
		for i := 0; i <= r.NumberOfInterval-1; i++ {
			accumulatedProfit := r.accumulatedProfitPerDay.Index(r.IntervalWindow * i)
			accumulatedProfitStr := fmt.Sprintf("%f", accumulatedProfit)
			accumulatedProfitMA := r.accumulatedProfitMAPerDay.Index(r.IntervalWindow * i)
			accumulatedProfitMAStr := fmt.Sprintf("%f", accumulatedProfitMA)
			intervalAccumulatedProfit := r.dailyProfit.Tail(r.AccumulatedDailyProfitWindow+r.IntervalWindow*i).Sum() - r.dailyProfit.Tail(r.IntervalWindow*i).Sum()
			intervalAccumulatedProfitStr := fmt.Sprintf("%f", intervalAccumulatedProfit)
			accumulatedFee := fmt.Sprintf("%f", r.accumulatedFeePerDay.Index(r.IntervalWindow*i))
			winRatio := fmt.Sprintf("%f", r.winRatioPerDay.Index(r.IntervalWindow*i))
			profitFactor := fmt.Sprintf("%f", r.profitFactorPerDay.Index(r.IntervalWindow*i))
			trades := r.dailyTrades.Tail(60+r.IntervalWindow*i).Sum() - r.dailyTrades.Tail(r.IntervalWindow*i).Sum()
			tradesStr := fmt.Sprintf("%f", trades)

			_ = tsvwiter.Write([]string{fmt.Sprintf("%d", i+1), symbol, accumulatedProfitStr, accumulatedProfitMAStr, intervalAccumulatedProfitStr, accumulatedFee, winRatio, profitFactor, tradesStr})
		}
	}
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

	// Accumulated profit report
	AccumulatedProfitReport *AccumulatedProfitReport `json:"accumulatedProfitReport"`
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

	// AccountValueCalculator
	s.AccountValueCalculator = bbgo.NewAccountValueCalculator(s.session, s.Market.QuoteCurrency)

	// Accumulated profit report
	if bbgo.IsBackTesting {
		if s.AccumulatedProfitReport == nil {
			s.AccumulatedProfitReport = &AccumulatedProfitReport{}
		}
		s.AccumulatedProfitReport.Initialize()
		s.orderExecutor.TradeCollector().OnProfit(func(trade types.Trade, profit *types.Profit) {
			if profit == nil {
				return
			}

			s.AccumulatedProfitReport.RecordProfit(profit.Profit)
		})
		// s.orderExecutor.TradeCollector().OnTrade(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
		// 	s.AccumulatedProfitReport.RecordTrade(trade.Fee)
		// })
		session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1d, func(kline types.KLine) {
			s.AccumulatedProfitReport.DailyUpdate(s.TradeStats)
		}))
	}

	// For drawing
	profitSlice := floats.Slice{1., 1.}
	price, _ := session.LastPrice(s.Symbol)
	initAsset := s.CalcAssetValue(price).Float64()
	cumProfitSlice := floats.Slice{initAsset, initAsset}

	s.orderExecutor.TradeCollector().OnTrade(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
		if bbgo.IsBackTesting {
			s.AccumulatedProfitReport.RecordTrade(trade.Fee)
		}

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

			orderForm := s.generateOrderForm(side, s.calculateQuantity(ctx, closePrice, side), types.SideEffectTypeMarginBuy)
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

		// Output accumulated profit report
		if bbgo.IsBackTesting {
			defer s.AccumulatedProfitReport.Output(s.Symbol)

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
