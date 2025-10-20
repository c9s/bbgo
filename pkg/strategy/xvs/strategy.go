package xvs

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "xvs"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

// Strategy XVS strategy main structure
// Based on high volume breakout entry signals and engulfing pattern exit signals
type Strategy struct {
	*common.Strategy

	Market      types.Market
	Environment *bbgo.Environment

	Symbol            string                 `json:"symbol"`
	Quantity          fixedpoint.Value       `json:"quantity"`
	ScaleQuantity     *bbgo.PriceVolumeScale `json:"scaleQuantity"`
	MaxExposure       fixedpoint.Value       `json:"maxExposure"`
	PriceRatioProtect fixedpoint.Value       `json:"priceRatioProtect"` // Price protection ratio (e.g. 1.05 means 5% slippage)

	// Entry condition parameters
	VolumeInterval           types.Interval   `json:"volumeInterval"`           // Volume monitoring time interval (e.g. 5m or 15m)
	VolumeThreshold          fixedpoint.Value `json:"volumeThreshold"`          // Base asset volume threshold (e.g. >= 800 BTC)
	VolumeThresholdInQuote   fixedpoint.Value `json:"volumeThresholdInQuote"`   // Quote asset volume threshold (e.g. > 40M USDT)
	MinKLineLowerShadowRatio fixedpoint.Value `json:"minKLineLowerShadowRatio"` // Minimum lower shadow ratio (0.1 means 90% body)

	// EMA technical indicator settings
	LongTermEMAWindow  types.IntervalWindow `json:"longTermEMAWindow"`  // Long-term EMA window settings, customizable days and period
	ShortTermEMAWindow types.IntervalWindow `json:"shortTermEMAWindow"` // Short-term EMA window settings, customizable days and period

	// Pivot High settings
	PivotHighWindow types.IntervalWindow `json:"pivotHighWindow"` // Pivot High window settings

	// Exit condition parameters
	EngulfingInterval types.Interval `json:"engulfingInterval"` // Engulfing pattern monitoring time interval (30m)

	// Exit methods collection (supports stop-loss and other exit methods)
	ExitMethods bbgo.ExitMethodSet `json:"exits"`

	// Technical indicators
	pivotHigh    *indicatorv2.PivotHighStream // Pivot high indicator
	longTermEMA  *indicatorv2.EWMAStream      // Long-term EMA exponential moving average
	shortTermEMA *indicatorv2.EWMAStream      // Short-term EMA exponential moving average

	// Engulfing pattern tracking
	lastGreenKline *types.KLine // Record last green kline for engulfing pattern detection

	// Persistent data
	TradeStats *types.TradeStats `persistence:"trade_stats"` // Trading statistics

	// Strategy controller (supports pause, emergency stop and other control functions)
	bbgo.StrategyController
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

// Validate validates the validity of strategy parameters
func (s *Strategy) Validate() error {
	if s.Quantity.IsZero() && s.ScaleQuantity == nil {
		return fmt.Errorf("quantity or scaleQuantity can not be zero")
	}
	// At least one volume threshold must be set
	if s.VolumeThreshold.IsZero() && s.VolumeThresholdInQuote.IsZero() {
		return fmt.Errorf("either volumeThreshold or volumeThresholdInQuote must be set")
	}
	// Maximum exposure must be set
	if s.MaxExposure.IsZero() {
		return fmt.Errorf("maxExposure must be positive")
	}
	return nil
}

// Defaults sets default values for strategy parameters
func (s *Strategy) Defaults() error {
	// Default monitor 5-minute kline volume
	if s.VolumeInterval == "" {
		s.VolumeInterval = types.Interval5m
	}

	// Default monitor 30-minute kline engulfing pattern
	if s.EngulfingInterval == "" {
		s.EngulfingInterval = types.Interval30m
	}

	// Default minimum lower shadow ratio is 50% (i.e. body occupies 50%)
	if s.MinKLineLowerShadowRatio.IsZero() {
		s.MinKLineLowerShadowRatio = fixedpoint.NewFromFloat(0.5)
	}

	// Default use 120 window with volume interval for long term EMA
	if s.LongTermEMAWindow.Interval == "" {
		s.LongTermEMAWindow.Interval = s.VolumeInterval
	}

	if s.LongTermEMAWindow.Window == 0 {
		s.LongTermEMAWindow.Window = 120
	}

	// Default use 20 window with volume interval for short term EMA
	if s.ShortTermEMAWindow.Interval == "" {
		s.ShortTermEMAWindow.Interval = s.VolumeInterval
	}

	if s.ShortTermEMAWindow.Window == 0 {
		s.ShortTermEMAWindow.Window = 20
	}

	// Default Pivot High settings
	if s.PivotHighWindow.Interval == "" {
		s.PivotHighWindow.Interval = s.VolumeInterval
	}

	if s.PivotHighWindow.Window == 0 {
		s.PivotHighWindow.Window = 5 // Default check 5 points on left and right
	}

	return nil
}

func (s *Strategy) Initialize() error {
	if s.Strategy == nil {
		s.Strategy = &common.Strategy{}
	}

	return nil
}

// Subscribe subscribes to necessary market data
func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// Subscribe to kline data for volume monitoring interval (for entry signals)
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.VolumeInterval})

	// Subscribe to kline data for engulfing pattern monitoring interval (for exit signals)
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.EngulfingInterval})

	// Subscribe to kline data required for EMA indicators
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.LongTermEMAWindow.Interval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.ShortTermEMAWindow.Interval})

	// Subscribe to kline data required for Pivot High indicator
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.PivotHighWindow.Interval})

	// Subscribe to data required for exit methods
	s.ExitMethods.SetAndSubscribe(session, s)
}

func (s *Strategy) CurrentPosition() *types.Position {
	return s.Position
}

// ClosePosition closes the specified percentage of position
func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	base := s.Position.GetBase()
	if base.IsZero() {
		return fmt.Errorf("no opened %s position", s.Position.Symbol)
	}

	// Calculate close quantity
	quantity := base.Mul(percentage).Abs()
	side := types.SideTypeBuy
	if base.Sign() > 0 {
		side = types.SideTypeSell
	}

	// Check minimum quantity requirement
	if quantity.Compare(s.Market.MinQuantity) < 0 {
		return fmt.Errorf("order quantity %v is too small, less than %v", quantity, s.Market.MinQuantity)
	}

	submitOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     side,
		Type:     types.OrderTypeMarket,
		Quantity: quantity,
		Market:   s.Market,
	}

	bbgo.Notify("Submitting %s %s order to close position by %v", s.Symbol, side.String(), percentage, submitOrder)
	_, err := s.OrderExecutor.SubmitOrders(ctx, submitOrder)
	return err
}

// checkVolumeBreakout checks if high volume breakout occurs
func (s *Strategy) checkVolumeBreakout(kline types.KLine) bool {
	// Check base asset volume threshold (e.g. >= 800 BTC)
	if !s.VolumeThreshold.IsZero() && kline.Volume.Compare(s.VolumeThreshold) < 0 {
		return false
	}

	// Check quote asset volume threshold (e.g. > 40M USDT)
	if !s.VolumeThresholdInQuote.IsZero() && kline.QuoteVolume.Compare(s.VolumeThresholdInQuote) < 0 {
		return false
	}

	// Check kline lower shadow ratio
	totalSize := kline.GetHigh().Sub(kline.GetLow()) // Total kline size
	if totalSize.IsZero() {
		return false
	}

	lowerShadowSize := fixedpoint.Min(kline.GetOpen(), kline.GetClose()).Sub(kline.GetLow()) // Kline lower shadow size
	lowerShadowRatio := lowerShadowSize.Div(totalSize)
	if lowerShadowRatio.Compare(s.MinKLineLowerShadowRatio) < 0 {
		log.Infof("Kline has too small lower shadow ratio %s, required >= %s", lowerShadowRatio, s.MinKLineLowerShadowRatio)
		return false
	}

	return true
}

// checkEMAFilter checks EMA trend filter conditions
func (s *Strategy) checkEMAFilter(closePrice fixedpoint.Value) bool {
	if s.longTermEMA == nil || s.shortTermEMA == nil {
		return false
	}

	longTermEMAValue := fixedpoint.NewFromFloat(s.longTermEMA.Last(0))
	shortTermEMAValue := fixedpoint.NewFromFloat(s.shortTermEMA.Last(0))
	// If price falls below long term EMA, skip entry
	if closePrice.Compare(longTermEMAValue) < 0 {
		log.Infof("Price %s is below long term EMA %s, skipping entry", closePrice, longTermEMAValue)
		return false
	}

	if closePrice.Compare(shortTermEMAValue) > 0 {
		log.Infof("Price %s is above short term EMA %s, skipping entry", closePrice, shortTermEMAValue)
		return false
	}

	return true
}

// checkEngulfingExit checks engulfing pattern exit conditions
func (s *Strategy) checkEngulfingExit(kline types.KLine) bool {
	if s.lastGreenKline == nil {
		return false
	}
	log.Infof("[Exit] Checking engulfing exit for kline %s, last green kline %s", kline.String(), s.lastGreenKline.String())

	// Current kline must be red and close price below previous green kline's open price
	if kline.GetClose().Compare(kline.GetOpen()) >= 0 {
		return false // Not a red kline
	}

	if kline.GetClose().Compare(s.lastGreenKline.GetOpen()) >= 0 {
		return false // Not below previous green kline's open price
	}

	// Volume must be greater than previous green kline
	if kline.Volume.Compare(s.lastGreenKline.Volume) <= 0 {
		return false
	}

	return true
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

	// initialize common strategy
	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, instanceID)
	s.OrderExecutor.BindTradeStats(s.TradeStats)

	indicatorSet := session.Indicators(s.Symbol)
	s.longTermEMA = indicatorSet.EWMA(s.LongTermEMAWindow)
	s.shortTermEMA = indicatorSet.EWMA(s.ShortTermEMAWindow)
	s.pivotHigh = indicatorv2.PivotHigh(indicatorSet.HIGH(s.PivotHighWindow.Interval), s.PivotHighWindow.Window)

	s.ExitMethods.Bind(session, s.OrderExecutor)

	s.Status = types.StrategyStatusRunning

	s.OnSuspend(func() {
		_ = s.OrderExecutor.GracefulCancel(ctx)
		bbgo.Sync(ctx, s)
	})

	// Listen to KLine closed event - Entry
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		s.EnterBuy(ctx, kline)
	})

	// Listen to KLine closed event - Take Profit
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		s.TakeProfit(ctx, kline)
	})

	// this callback must be registered last, because the callbacks are executed in order.
	// if registered earlier, it may affect the take profit condition.
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		s.setLastGreenKLine(&kline)
	})

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		_ = s.OrderExecutor.GracefulCancel(ctx)
	})

	return nil
}

func (s *Strategy) setLastGreenKLine(kline *types.KLine) {
	if s.Status != types.StrategyStatusRunning {
		return
	}

	// Kline is not for target symbol
	if kline.Symbol != s.Symbol {
		return
	}

	// Not the interval for entry
	if kline.Interval != s.EngulfingInterval {
		return
	}

	if kline.GetClose().Compare(kline.GetOpen()) < 0 {
		s.lastGreenKline = kline
	}
}

func (s *Strategy) EnterBuy(ctx context.Context, kline types.KLine) {
	// Strategy not running
	if s.Status != types.StrategyStatusRunning {
		return
	}

	// Kline is not for target symbol
	if kline.Symbol != s.Symbol {
		return
	}

	// Not the interval for entry
	if kline.Interval != s.VolumeInterval {
		return
	}

	// Maximum position exposure reached
	log.Infof("[Entry] Current position base %s, max exposure %s", s.Position.GetBase(), s.MaxExposure)
	if s.Position.GetBase().Compare(s.MaxExposure) >= 0 {
		return
	}

	// Check volume breakout conditions
	log.Infof("[Entry] Checking volume breakout for kline %s", kline.String())
	if !s.checkVolumeBreakout(kline) {
		return
	}

	closePrice := kline.GetClose()
	log.Infof("[Entry] Checking if closed price %f is above long term EMA %f and below short term EMA %f", closePrice.Float64(), s.longTermEMA.Last(0), s.shortTermEMA.Last(0))
	if !s.checkEMAFilter(closePrice) {
		return
	}

	bbgo.Notify("Found %s volume breakout: volume %s, quote volume %s, price %s",
		s.Symbol,
		kline.Volume.String(),
		kline.QuoteVolume.String(),
		closePrice.String(),
		kline)

	var quantity fixedpoint.Value
	if !s.Quantity.IsZero() {
		quantity = s.Quantity
	} else {
		if q, err := s.ScaleQuantity.Scale(closePrice.Float64(), kline.Volume.Float64()); err != nil {
			log.WithError(err).Error("scale quantity error")
			return
		} else {
			quantity = fixedpoint.NewFromFloat(q)
		}
	}
	orderForm := types.SubmitOrder{
		Symbol:   s.Symbol,
		Market:   s.Market,
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeMarket,
		Quantity: quantity,
	}

	if !s.PriceRatioProtect.IsZero() {
		orderForm.Type = types.OrderTypeLimit
		orderForm.Price = closePrice.Mul(fixedpoint.One.Add(s.PriceRatioProtect.Abs())) // Set protection price
		orderForm.TimeInForce = types.TimeInForceIOC
	}

	bbgo.Notify("Submitting %s market buy order with quantity %s",
		s.Symbol,
		s.Quantity.String(),
		orderForm)

	if _, err := s.OrderExecutor.SubmitOrders(ctx, orderForm); err != nil {
		log.WithError(err).Error("submit order error")
	}
}

func (s *Strategy) TakeProfit(ctx context.Context, kline types.KLine) {
	if s.Status != types.StrategyStatusRunning {
		return
	}

	// Kline is not for target symbol
	if kline.Symbol != s.Symbol {
		return
	}

	// Not the interval for exit
	if kline.Interval != s.EngulfingInterval {
		return
	}

	// Price below cost price, skip take profit exit check
	if s.Position.GetAverageCost().Compare(kline.GetClose()) >= 0 {
		return
	}

	// Only allow take profit exit when price exceeds pivot high
	if s.pivotHigh.Length() == 0 {
		return
	}

	currentPivotHigh := fixedpoint.NewFromFloat(s.pivotHigh.Last(0))
	if kline.GetClose().Compare(currentPivotHigh) <= 0 {
		log.Infof("[Exit] Price %s has not exceeded pivot high %s, skipping exit",
			kline.GetClose(), currentPivotHigh)
		return
	}

	log.Infof("[Exit] Price %s exceeded pivot high %s, allowing exit", kline.GetClose(), currentPivotHigh)

	log.Infof("[Exit] Current position base %s, min quantity %s", s.Position.GetBase(), s.Market.MinQuantity)
	if s.Position.GetBase().Compare(s.Market.MinQuantity) < 0 {
		return
	}

	// Check engulfing exit pattern
	if !s.checkEngulfingExit(kline) {
		return
	}

	bbgo.Notify("Found %s engulfing exit pattern: red kline %s with volume %s > previous green volume %s",
		s.Symbol,
		kline.GetClose().String(),
		kline.Volume.String(),
		s.lastGreenKline.Volume.String(),
		kline)

	// Close 100% position
	if err := s.ClosePosition(ctx, fixedpoint.One); err != nil {
		log.WithError(err).Error("close position error")
	}
}
