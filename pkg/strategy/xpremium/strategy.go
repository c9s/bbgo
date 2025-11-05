package xpremium

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "xpremium"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type BacktestConfig struct {
	BidAskPriceCsv  string         `json:"bidAskPriceCsv,omitempty"`
	TradingInterval types.Interval `json:"tradingInterval,omitempty"`
}

type backtestBidAsk struct {
	time                   time.Time
	baseAsk, baseBid       fixedpoint.Value
	premiumAsk, premiumBid fixedpoint.Value
}

type EngulfingTakeProfitConfig struct {
	Enabled              bool             `json:"enabled"`
	Interval             types.Interval   `json:"interval"`
	BodyMultiple         fixedpoint.Value `json:"bodyMultiple"`
	BottomShadowMaxRatio fixedpoint.Value `json:"bottomShadowMaxRatio"`
}

type PivotStopConfig struct {
	Enabled  bool           `json:"enabled"`
	Interval types.Interval `json:"interval"`
	Left     int            `json:"left"`
	Right    int            `json:"right"`
}

type Strategy struct {
	*common.Strategy

	Environment *bbgo.Environment

	// Symbol is the default trading pair for the trading session (fallback for TradingSymbol)
	Symbol string `json:"symbol"`

	// Premium session & symbol are the leading market to compare
	PremiumSession string `json:"premiumSession"`
	PremiumSymbol  string `json:"premiumSymbol"`

	// Base session & symbol are the lagging market to compare
	BaseSession string `json:"baseSession"`
	BaseSymbol  string `json:"baseSymbol"`

	// Trading session & symbol are where we open LONG/SHORT
	TradingSession string `json:"tradingSession"`
	TradingSymbol  string `json:"tradingSymbol"`

	// MinSpread is the minimum absolute price difference to trigger a signal (premium - base)
	MinSpread fixedpoint.Value `json:"minSpread"`

	// Leverage to set on the trading session (futures)
	MaxLeverage int `json:"leverage"`

	// Quantity is the fixed order size to trade on signal. If zero, sizing will be computed.
	Quantity fixedpoint.Value `json:"quantity"`

	// MaxLossLimit is the maximum quote loss allowed per trade for sizing; if zero, falls back to Quantity/min.
	MaxLossLimit fixedpoint.Value `json:"maxLossLimit"`

	// PriceType selects which price from ticker to use for sizing/validation (maker/taker)
	PriceType types.PriceType `json:"priceType"`

	// TakeProfitROI is the ROI threshold to take profit (e.g., 0.03 for 3%).
	TakeProfitROI fixedpoint.Value `json:"takeProfitROI"`
	// StopLossSafetyRatio is the adjustment ratio applied to previous pivot for stop loss
	// For long: stop = prevLow * (1 - ratio); for short: stop = prevHigh * (1 + ratio)
	StopLossSafetyRatio fixedpoint.Value `json:"stopLossSafetyRatio"`

	// EngulfingTakeProfit is an optional take-profit rule triggered by 1h Engulfing pattern
	EngulfingTakeProfit *EngulfingTakeProfitConfig `json:"engulfingTakeProfit,omitempty"`

	PivotStop *PivotStopConfig `json:"pivotStop,omitempty"`

	BacktestConfig *BacktestConfig `json:"backtest,omitempty"`

	logger        logrus.FieldLogger
	metricsLabels prometheus.Labels

	premiumSession, baseSession, tradingSession *bbgo.ExchangeSession
	tradingMarket                               types.Market

	// runtime fields
	premiumBook *types.StreamOrderBook
	baseBook    *types.StreamOrderBook

	premiumStream types.Stream
	baseStream    types.Stream

	// add connector manager to manage connectors/streams
	connectorManager *types.ConnectorManager

	// backtest data map keyed by minute-precision time
	btData map[time.Time]backtestBidAsk

	pvHigh *indicatorv2.PivotHighStream
	pvLow  *indicatorv2.PivotLowStream
	kLines *indicatorv2.KLineStream

	lastTPCheck time.Time
}

type Signal struct {
	Side       types.SideType
	Premium    fixedpoint.Value
	Discount   fixedpoint.Value
	MinSpread  fixedpoint.Value
	PremiumBid fixedpoint.Value
	PremiumAsk fixedpoint.Value
	BaseBid    fixedpoint.Value
	BaseAsk    fixedpoint.Value
	Symbol     string
}

func (s *Signal) SlackAttachment() slack.Attachment {
	color := "good"
	if s.Side == types.SideTypeSell {
		color = "danger"
	}

	return slack.Attachment{
		Title: fmt.Sprintf("XPremium %s Signal", s.Side.String()),
		Color: color,
		Fields: []slack.AttachmentField{
			{
				Title: "Symbol",
				Value: s.Symbol,
				Short: true,
			},
			{
				Title: "Premium",
				Value: fmt.Sprintf("%.4f%%", s.Premium.Float64()*100),
				Short: true,
			},
			{
				Title: "Discount",
				Value: fmt.Sprintf("%.4f%%", s.Discount.Float64()*100),
				Short: true,
			},
			{
				Title: "Min Spread",
				Value: fmt.Sprintf("%.4f%%", s.MinSpread.Float64()*100),
				Short: true,
			},
			{
				Title: "Premium Bid/Ask",
				Value: fmt.Sprintf("%s / %s", s.PremiumBid.String(), s.PremiumAsk.String()),
				Short: false,
			},
			{
				Title: "Base Bid/Ask",
				Value: fmt.Sprintf("%s / %s", s.BaseBid.String(), s.BaseAsk.String()),
				Short: false,
			},
		},
	}
}

func (s *Strategy) ID() string { return ID }

func (s *Strategy) InstanceID() string {
	return strings.Join([]string{ID, s.BaseSession, s.PremiumSession, s.Symbol}, ":")
}

func (s *Strategy) Initialize() error {
	if s.Strategy == nil {
		s.Strategy = &common.Strategy{}
	}

	s.logger = logrus.WithFields(logrus.Fields{
		"symbol":      s.Symbol,
		"strategy":    ID,
		"strategy_id": s.InstanceID(),
	})

	s.metricsLabels = prometheus.Labels{
		"strategy_type":   ID,
		"strategy_id":     s.InstanceID(),
		"base_session":    s.BaseSession,
		"premium_session": s.PremiumSession,
		"symbol":          s.Symbol,
	}

	// initialize connector manager
	s.connectorManager = types.NewConnectorManager()
	return nil
}

func (s *Strategy) Defaults() error {
	// default trading session to premium session if not specified
	if s.TradingSession == "" {
		s.TradingSession = s.PremiumSession
	}
	// default trading symbol to Symbol, then PremiumSymbol
	if s.TradingSymbol == "" {
		if s.Symbol != "" {
			s.TradingSymbol = s.Symbol
		} else if s.PremiumSymbol != "" {
			s.TradingSymbol = s.PremiumSymbol
		}
	}

	// ensure Symbol has a value for logging/metrics/instance id
	if s.Symbol == "" {
		if s.TradingSymbol != "" {
			s.Symbol = s.TradingSymbol
		} else if s.PremiumSymbol != "" {
			s.Symbol = s.PremiumSymbol
		}
	}
	// default price type
	if s.PriceType == "" {
		s.PriceType = types.PriceTypeMaker
	}

	if s.MaxLossLimit.IsZero() {
		s.MaxLossLimit = fixedpoint.NewFromInt(20) // default to 100 units of quote currency
	}

	// default take profit ROI to 3%
	if s.TakeProfitROI.IsZero() {
		s.TakeProfitROI = fixedpoint.NewFromFloat(0.03)
	}
	// default stop loss safety ratio to 1%
	if s.StopLossSafetyRatio.IsZero() {
		s.StopLossSafetyRatio = fixedpoint.NewFromFloat(0.01)
	}

	if s.PivotStop == nil {
		s.PivotStop = &PivotStopConfig{
			Enabled:  false,
			Interval: types.Interval15m,
			Left:     10,
			Right:    10,
		}
	} else if s.PivotStop.Enabled {
		if s.PivotStop.Interval == "" {
			s.PivotStop.Interval = types.Interval15m
		}
		if s.PivotStop.Left == 0 {
			s.PivotStop.Left = 10
		}

		if s.PivotStop.Right == 0 {
			s.PivotStop.Right = 10
		}
	}

	// defaults for engulfing take profit
	if s.EngulfingTakeProfit == nil {
		// initialize with sensible defaults; remains disabled unless explicitly enabled
		s.EngulfingTakeProfit = &EngulfingTakeProfitConfig{
			Enabled:              false,
			Interval:             types.Interval1h,
			BodyMultiple:         fixedpoint.NewFromFloat(1.0),
			BottomShadowMaxRatio: fixedpoint.Zero, // disabled by default
		}
	} else {
		if s.EngulfingTakeProfit.Interval == "" {
			s.EngulfingTakeProfit.Interval = types.Interval1h
		}

		if s.EngulfingTakeProfit.BodyMultiple.IsZero() {
			// default: at least the same size as previous body
			s.EngulfingTakeProfit.BodyMultiple = fixedpoint.NewFromFloat(1.0)
		}

		// default bottom shadow max ratio to 0 (disabled) if not provided or negative
		if s.EngulfingTakeProfit.BottomShadowMaxRatio.Sign() < 0 {
			s.EngulfingTakeProfit.BottomShadowMaxRatio = fixedpoint.Zero
		}
	}

	return nil
}

func (s *Strategy) Validate() error {
	if s.PremiumSession == "" {
		return fmt.Errorf("premiumSession is required")
	}
	if s.BaseSession == "" {
		return fmt.Errorf("baseSession is required")
	}
	if s.PremiumSymbol == "" {
		return fmt.Errorf("premiumSymbol is required")
	}
	if s.BaseSymbol == "" {
		return fmt.Errorf("baseSymbol is required")
	}
	if s.TradingSession == "" {
		return fmt.Errorf("tradingSession is required")
	}
	if s.TradingSymbol == "" {
		return fmt.Errorf("tradingSymbol is required")
	}
	if s.MinSpread.IsZero() {
		return fmt.Errorf("minSpread must be greater than 0")
	}
	if s.MaxLeverage < 0 {
		return fmt.Errorf("leverage must be >= 0")
	}
	if s.TakeProfitROI.Sign() < 0 {
		return fmt.Errorf("takeProfitROI must be >= 0")
	}
	if s.StopLossSafetyRatio.Sign() < 0 {
		return fmt.Errorf("stopLossSafetyRatio must be >= 0")
	}

	if s.EngulfingTakeProfit != nil {
		if s.EngulfingTakeProfit.BodyMultiple.Sign() < 0 {
			return fmt.Errorf("engulfingTakeProfit.bodyMultiple must be >= 0")
		}
		if s.EngulfingTakeProfit.BottomShadowMaxRatio.Sign() < 0 {
			return fmt.Errorf("engulfingTakeProfit.bottomShadowMaxRatio must be >= 0")
		}
	}
	return nil
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	tradingSession := sessions[s.TradingSession]

	if premiumSession, ok := sessions[s.PremiumSession]; ok {
		premiumSession.Subscribe(types.KLineChannel, s.PremiumSymbol, types.SubscribeOptions{Interval: types.Interval1m})
		tradingSession.Subscribe(types.KLineChannel, s.TradingSymbol, types.SubscribeOptions{Interval: types.Interval1m})

		premiumSession.Subscribe(types.KLineChannel, s.PremiumSymbol, types.SubscribeOptions{Interval: types.Interval15m})
		tradingSession.Subscribe(types.KLineChannel, s.TradingSymbol, types.SubscribeOptions{Interval: types.Interval15m})

		if s.EngulfingTakeProfit != nil && s.EngulfingTakeProfit.Enabled {
			interval := s.EngulfingTakeProfit.Interval
			premiumSession.Subscribe(types.KLineChannel, s.PremiumSymbol, types.SubscribeOptions{Interval: interval})
			tradingSession.Subscribe(types.KLineChannel, s.TradingSymbol, types.SubscribeOptions{Interval: interval})
		}

		if s.PivotStop != nil && s.PivotStop.Enabled {
			interval := s.PivotStop.Interval
			premiumSession.Subscribe(types.KLineChannel, s.PremiumSymbol, types.SubscribeOptions{Interval: interval})
			tradingSession.Subscribe(types.KLineChannel, s.TradingSymbol, types.SubscribeOptions{Interval: interval})
		}
	}
}

func (s *Strategy) CrossRun(ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	if bbgo.IsBackTesting {
		return nil
	}

	// Defaults() and Validate() should have been called prior to CrossRun,
	// so we assume required fields are populated here.
	ok := false
	s.premiumSession, ok = sessions[s.PremiumSession]
	if !ok {
		return fmt.Errorf("premium session %s not found", s.PremiumSession)
	}

	s.baseSession, ok = sessions[s.BaseSession]
	if !ok {
		return fmt.Errorf("base session %s not found", s.BaseSession)
	}

	s.tradingSession, ok = sessions[s.TradingSession]
	if !ok {
		return fmt.Errorf("trading session %s not found", s.TradingSession)
	}
	tradingSymbol := s.TradingSymbol

	// initialize common.Strategy with trading session and market to use Position, ProfitStats and GeneralOrderExecutor
	tradingMarket, ok := s.tradingSession.Market(tradingSymbol)
	if !ok {
		return fmt.Errorf("trading session market %s is not defined", tradingSymbol)
	}
	// keep a reference to trading market
	s.tradingMarket = tradingMarket

	// Initialize the core strategy components (Position, ProfitStats, GeneralOrderExecutor)
	s.Strategy.Initialize(ctx, s.Environment, s.tradingSession, tradingMarket, ID, s.InstanceID())

	s.pvLow = indicatorv2.PivotLow(s.tradingSession.Indicators(s.TradingSymbol).LOW(s.PivotStop.Interval), s.PivotStop.Left, s.PivotStop.Right)
	s.pvHigh = indicatorv2.PivotHigh(s.tradingSession.Indicators(s.TradingSymbol).HIGH(s.PivotStop.Interval), s.PivotStop.Left, s.PivotStop.Right)
	s.kLines = s.tradingSession.Indicators(s.TradingSymbol).KLines(s.PivotStop.Interval)

	// set leverage if configured and supported

	if riskSvc, ok := s.tradingSession.Exchange.(types.ExchangeRiskService); ok {
		if s.MaxLeverage > 0 {
			if err := riskSvc.SetLeverage(ctx, tradingSymbol, s.MaxLeverage); err != nil {
				s.logger.WithError(err).Errorf("failed to set leverage to %d on %s", s.MaxLeverage, tradingSymbol)
			} else {
				s.logger.Infof("leverage set to %d on %s", s.MaxLeverage, tradingSymbol)
			}
		}

		if err := s.syncPositionRisks(ctx, riskSvc, tradingSymbol); err != nil {
			s.logger.WithError(err).Errorf("failed to sync position risks on startup")
		}
	}

	// register engulfing take-profit handler on kline close
	if s.EngulfingTakeProfit != nil && s.EngulfingTakeProfit.Enabled {
		interval := s.EngulfingTakeProfit.Interval
		s.premiumSession.MarketDataStream.OnKLineClosed(types.KLineWith(s.PremiumSymbol, interval, func(k types.KLine) {
			s.maybeEngulfingTakeProfit(ctx, k)
		}))
	}

	s.premiumSession.MarketDataStream.OnKLine(types.KLineWith(s.PremiumSymbol, types.Interval1m, func(k types.KLine) {
		// backtest ROI take-profit using kline close
		if _, err := s.maybeRoiTakeProfit(ctx, k.GetClose()); err != nil {
			s.logger.WithError(err).Warn("take-profit error")
		}
	}))

	// allocate isolated public streams for books and bind StreamBooks
	premiumStream := bbgo.NewBookStream(s.premiumSession, s.PremiumSymbol)
	baseStream := bbgo.NewBookStream(s.baseSession, s.BaseSymbol)

	s.premiumStream, s.baseStream = premiumStream, baseStream

	s.premiumBook = types.NewStreamBook(s.PremiumSymbol, s.premiumSession.ExchangeName)
	s.premiumBook.BindStream(premiumStream)

	s.baseBook = types.NewStreamBook(s.BaseSymbol, s.baseSession.ExchangeName)
	s.baseBook.BindStream(baseStream)

	// register streams into the connector manager and connect them via connector manager
	s.connectorManager.Add(premiumStream, baseStream)

	if err := s.connectorManager.Connect(ctx); err != nil {
		s.logger.WithError(err).Error("connector manager connect error")
		return err
	}

	// wait for both sessions' user data streams to be authenticated before starting the premium worker
	group := types.NewConnectivityGroup(
		s.premiumSession.UserDataConnectivity,
		s.baseSession.UserDataConnectivity,
	)

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		_ = s.OrderExecutor.GracefulCancel(ctx)

		bbgo.Sync(ctx, s)

		_, _ = fmt.Fprintln(os.Stderr, s.TradeStats.String())
	})

	go func() {
		s.logger.Infof("waiting for authentication of premium and base sessions...")
		select {
		case <-ctx.Done():
			return
		case <-group.AllAuthedC(ctx):
		}

		s.logger.Infof("both premium and base sessions authenticated, starting premium worker")

		s.premiumWorker(ctx)
	}()

	return nil
}

// computeSpreads implements the bid-ask percentage comparison algorithm:
// premium  = (premiumBid - baseAsk) / baseAsk
// discount = (premiumAsk - baseBid) / premiumAsk
func (s *Strategy) computeSpreads(pBid, pAsk, bBid, bAsk types.PriceVolume) (premium, discount float64) {
	// avoid division by zero
	if !bAsk.Price.IsZero() {
		premium = pBid.Price.Sub(bAsk.Price).Div(bAsk.Price).Float64()
	}

	if !pAsk.Price.IsZero() {
		discount = pAsk.Price.Sub(bBid.Price).Div(pAsk.Price).Float64()
	}

	return premium, discount
}

// compareBooks fetches best bid/ask from both books and returns spreads
func (s *Strategy) compareBooks() (premium, discount float64, pBid, pAsk, bBid, bAsk types.PriceVolume, ok bool) {
	bidA, askA, okA := s.premiumBook.BestBidAndAsk()
	bidB, askB, okB := s.baseBook.BestBidAndAsk()
	if !okA || !okB {
		return 0.0, 0.0, types.PriceVolume{}, types.PriceVolume{}, types.PriceVolume{}, types.PriceVolume{}, false
	}

	prem, disc := s.computeSpreads(bidA, askA, bidB, askB)
	return prem, disc, bidA, askA, bidB, askB, true
}

// decideSignal determines LONG when premium >= MinSpread, SHORT when discount <= -MinSpread
func (s *Strategy) decideSignal(premium, discount float64) types.SideType {
	if s.MinSpread.IsZero() {
		s.logger.Warn("min spread is not configured, skipping signal decision")
		return ""
	}

	if premium >= s.MinSpread.Float64() {
		return types.SideTypeBuy
	}
	if discount <= -s.MinSpread.Float64() {
		return types.SideTypeSell
	}
	return ""
}

// validateStopPrice ensures the stop price is on the correct side of the current price per side.
// For long: stop < current. For short: stop > current.
func (s *Strategy) validateStopPrice(side types.SideType, currentPrice, stopPrice fixedpoint.Value) (bool, error) {
	if stopPrice.Sign() <= 0 || currentPrice.IsZero() {
		return false, fmt.Errorf("invalid price: current=%s stop=%s", currentPrice.String(), stopPrice.String())
	}
	if side == types.SideTypeBuy {
		if stopPrice.Compare(currentPrice) >= 0 {
			return false, fmt.Errorf("stop loss must be below current price for long: stop=%s current=%s", stopPrice.String(), currentPrice.String())
		}
		return true, nil
	}
	if stopPrice.Compare(currentPrice) <= 0 {
		return false, fmt.Errorf("stop loss must be above current price for short: stop=%s current=%s", stopPrice.String(), currentPrice.String())
	}
	return true, nil
}

// findPivotStop derives stop from last detected pivot low/high with safety ratio applied.
// Returns zero if no pivot is available.
func (s *Strategy) findPivotStop(side types.SideType) fixedpoint.Value {
	safetyDown := fixedpoint.One.Sub(s.StopLossSafetyRatio)
	safetyUp := fixedpoint.One.Add(s.StopLossSafetyRatio)
	switch side {
	case types.SideTypeBuy:
		if s.pvLow != nil && s.pvLow.Length() > 0 {
			return fixedpoint.NewFromFloat(s.pvLow.Last(0)).Mul(safetyDown)
		}
	case types.SideTypeSell:
		if s.pvHigh != nil && s.pvHigh.Length() > 0 {
			return fixedpoint.NewFromFloat(s.pvHigh.Last(0)).Mul(safetyUp)
		}
	}
	return fixedpoint.Zero
}

// findNearestStop derives stop from the most recent closed kline high/low with safety ratio.
func (s *Strategy) findNearestStop(ctx context.Context, side types.SideType, now time.Time, n int) (fixedpoint.Value, error) {
	// helper to compute with kline window utilities
	computeFromKLines := func(kw types.KLineWindow) (fixedpoint.Value, error) {
		if kw.Len() == 0 {
			return fixedpoint.Zero, fmt.Errorf("no klines to compute stop price")
		}
		// ensure last kline is not in the future relative to now
		last := kw.Last()
		if last.EndTime.Time().After(now) {
			return fixedpoint.Zero, fmt.Errorf("latest kline ends after now %s > %s", last.EndTime.Time(), now)
		}

		safetyDown := fixedpoint.One.Sub(s.StopLossSafetyRatio)
		safetyUp := fixedpoint.One.Add(s.StopLossSafetyRatio)
		if side == types.SideTypeBuy {
			low := kw.GetLow()
			return low.Mul(safetyDown), nil
		}
		high := kw.GetHigh()
		return high.Mul(safetyUp), nil
	}

	// Prefer in-memory kline stream if available
	if s.kLines != nil && s.kLines.Length() > 0 {
		kw := s.kLines.Tail(n)
		if kw.Len() > 0 {
			if sp, err := computeFromKLines(kw); err == nil && sp.Sign() > 0 {
				return sp, nil
			}
		}
	}

	// Fallback to querying klines from the trading exchange
	if s.tradingSession == nil || s.tradingSession.Exchange == nil {
		return fixedpoint.Zero, fmt.Errorf("trading session not initialized for kline query")
	}

	interval := s.PivotStop.Interval
	if interval == "" {
		interval = types.Interval15m
	}

	end := now
	kls, err := s.tradingSession.Exchange.QueryKLines(ctx,
		s.TradingSymbol, interval,
		types.KLineQueryOptions{EndTime: &end, Limit: n})

	if err != nil {
		return fixedpoint.Zero, err
	}

	if len(kls) == 0 {
		return fixedpoint.Zero, fmt.Errorf("no klines returned from exchange for stop loss calculation")
	}

	return computeFromKLines(kls)
}

// getRatioStop derives stop directly from current price and safety ratio (1% default).
// For long: price * (1 - r); for short: price * (1 + r)
func (s *Strategy) getRatioStop(currentPrice fixedpoint.Value, side types.SideType) fixedpoint.Value {
	if currentPrice.IsZero() {
		return fixedpoint.Zero
	}
	if side == types.SideTypeBuy {
		return currentPrice.Mul(fixedpoint.One.Sub(s.StopLossSafetyRatio))
	}
	return currentPrice.Mul(fixedpoint.One.Add(s.StopLossSafetyRatio))
}

// findStopPrice tries multiple methods to determine a valid stop price:
// 1) Pivot-based, 2) Nearest kline high/low, 3) Ratio from current price.
func (s *Strategy) findStopPrice(ctx context.Context, ticker *types.Ticker, side types.SideType, now time.Time) (fixedpoint.Value, error) {
	// resolve current price first
	currentPrice := ticker.GetPrice(side, s.PriceType)
	if currentPrice.IsZero() {
		currentPrice = ticker.GetValidPrice()
	}
	if currentPrice.IsZero() {
		return fixedpoint.Zero, fmt.Errorf("invalid current price")
	}

	// 1) Pivot-based
	if sp := s.findPivotStop(side); sp.Sign() > 0 {
		if ok, _ := s.validateStopPrice(side, currentPrice, sp); ok {
			return sp, nil
		}
	}

	// 2) Nearest kline high/low from the recent 3 bars
	if sp, err := s.findNearestStop(ctx, side, now, 3); err == nil && sp.Sign() > 0 {
		if ok, _ := s.validateStopPrice(side, currentPrice, sp); ok {
			return sp, nil
		}
	}

	// 3) Ratio fallback
	if sp := s.getRatioStop(currentPrice, side); sp.Sign() > 0 {
		if ok, _ := s.validateStopPrice(side, currentPrice, sp); ok {
			return sp, nil
		}
	}

	return fixedpoint.Zero, fmt.Errorf("unable to determine a valid stop price")
}

// calculatePositionSize sizes order using MaxLossLimit and stop loss like tradingdesk
func (s *Strategy) calculatePositionSize(ctx context.Context, side types.SideType, stopLoss fixedpoint.Value) (fixedpoint.Value, error) {
	// If MaxLossLimit is zero or stopLoss invalid, fallback to configured Quantity or min
	if s.MaxLossLimit.IsZero() || stopLoss.IsZero() {
		if s.Quantity.Sign() > 0 {
			return s.Quantity, nil
		}
		return s.tradingMarket.MinQuantity, nil
	}

	ticker, err := s.tradingSession.Exchange.QueryTicker(ctx, s.TradingSymbol)
	if err != nil {
		return fixedpoint.Zero, err
	}
	currentPrice := s.PriceType.GetPrice(ticker, side)
	if currentPrice.IsZero() {
		currentPrice = ticker.GetValidPrice()
	}
	if currentPrice.IsZero() {
		return fixedpoint.Zero, fmt.Errorf("invalid current price")
	}

	// validate stop relative to price using shared validator
	if ok, err := s.validateStopPrice(side, currentPrice, stopLoss); !ok {
		return fixedpoint.Zero, err
	}

	var riskPerUnit fixedpoint.Value
	if side == types.SideTypeBuy {
		riskPerUnit = currentPrice.Sub(stopLoss)
	} else {
		riskPerUnit = stopLoss.Sub(currentPrice)
	}

	if riskPerUnit.Sign() <= 0 {
		return fixedpoint.Zero, fmt.Errorf("invalid risk per unit, computed as %s", riskPerUnit)
	}

	maxQtyByRisk := s.MaxLossLimit.Div(riskPerUnit)

	// balance constraint
	account := s.tradingSession.GetAccount()
	var maxQtyByBalance fixedpoint.Value
	if s.tradingSession.Futures {
		quoteBal, ok := account.Balance(s.tradingMarket.QuoteCurrency)
		if !ok {
			return fixedpoint.Zero, fmt.Errorf("no %s balance", s.tradingMarket.QuoteCurrency)
		}
		maxQtyByBalance = quoteBal.Available.Mul(fixedpoint.NewFromInt(int64(s.MaxLeverage))).Div(currentPrice)
	} else {
		// TODO: use accountValueCalculator to calculate the position size that we can borrow from the margin account.
		accountValueCalculator := s.tradingSession.GetAccountValueCalculator()
		_ = accountValueCalculator

		if side == types.SideTypeBuy {
			quoteBal, ok := account.Balance(s.tradingMarket.QuoteCurrency)
			if !ok {
				return fixedpoint.Zero, fmt.Errorf("no %s balance", s.tradingMarket.QuoteCurrency)
			}
			maxQtyByBalance = quoteBal.Available.Div(currentPrice)
		} else {
			baseBal, ok := account.Balance(s.tradingMarket.BaseCurrency)
			if !ok {
				return fixedpoint.Zero, fmt.Errorf("no %s balance", s.tradingMarket.BaseCurrency)
			}
			maxQtyByBalance = baseBal.Available
		}
	}

	qty := fixedpoint.Min(maxQtyByRisk, maxQtyByBalance)
	// apply market constraints
	qty = s.tradingMarket.TruncateQuantity(qty)
	if qty.Compare(s.tradingMarket.MinQuantity) < 0 {
		qty = s.tradingMarket.MinQuantity
	}
	qty = s.tradingMarket.AdjustQuantityByMinNotional(qty, currentPrice)
	return qty, nil
}

func (s *Strategy) ensureOppositePositionClosed(ctx context.Context, side types.SideType) error {
	if s.Position == nil {
		return nil
	}

	if s.Position.GetBase().IsZero() {
		return nil
	}

	if err := s.OrderExecutor.GracefulCancel(ctx); err != nil {
		s.logger.WithError(err).Warnf("graceful order cancel error before ensuring opposite position closed")
	}

	if (side == types.SideTypeBuy && s.Position.IsShort()) || (side == types.SideTypeSell && s.Position.IsLong()) {
		s.logger.Infof("closing opposite position before opening new %s position: current=%s", side, s.Position.String())

		if err := s.OrderExecutor.ClosePosition(ctx, fixedpoint.One); err != nil {
			return fmt.Errorf("close opposite position error: %w", err)
		}
	}

	return nil
}

// executeSignal submits a market order and a stop market order as stop loss
func (s *Strategy) executeSignal(ctx context.Context, side types.SideType, now time.Time) error {
	if side == "" || s.OrderExecutor == nil {
		return nil
	}

	// cancel any existing working orders first
	_ = s.OrderExecutor.GracefulCancel(ctx)

	// If we currently have a position on the opposite side, close it fully before opening a new one
	if err := s.ensureOppositePositionClosed(ctx, side); err != nil {
		return err
	}

	if s.Position.GetBase().Sign() > 0 && side == types.SideTypeBuy {
		s.logger.Infof("already have a LONG position, skipping new LONG entry")
		return nil
	} else if s.Position.GetBase().Sign() < 0 && side == types.SideTypeSell {
		s.logger.Infof("already have a SHORT position, skipping new SHORT entry")
		return nil
	}

	ticker, err := retry.QueryTickerUntilSuccessful(ctx, s.tradingSession.Exchange, s.TradingSymbol)
	if err != nil {
		return fmt.Errorf("query ticker with retry responds error: %w", err)
	}

	// derive stop loss from previous 15m kline
	stopLossPrice, err := s.findStopPrice(ctx, ticker, side, now)
	if err != nil {
		s.logger.WithError(err).Warn("failed to get 15m stop, fallback to quantity only")
	}

	qty, qerr := s.calculatePositionSize(ctx, side, stopLossPrice)
	if qerr != nil {
		s.logger.WithError(qerr).Warn("position sizing failed, fallback to min qty")
		qty = s.tradingMarket.MinQuantity
	}

	order := types.SubmitOrder{
		Symbol:   s.TradingSymbol,
		Side:     side,
		Type:     types.OrderTypeMarket,
		Quantity: qty,
		Market:   s.tradingMarket,

		Tag: "entry",
	}
	_, err = s.OrderExecutor.SubmitOrders(ctx, order)
	if err != nil {
		return err
	}

	// place stop loss if we have a valid stop
	if stopLossPrice.Sign() > 0 {
		stopOrder := types.SubmitOrder{
			Market:        s.tradingMarket,
			Symbol:        s.TradingSymbol,
			Side:          side.Reverse(),
			Type:          types.OrderTypeStopMarket,
			Quantity:      qty,
			StopPrice:     stopLossPrice,
			ClosePosition: true,

			Tag: "stop-loss",
		}
		if _, err := s.OrderExecutor.SubmitOrders(ctx, stopOrder); err != nil {
			s.logger.WithError(err).Warnf("submit stop loss order failed: %+v", stopOrder)
		}
	}

	return nil
}

// maybeRoiTakeProfit checks if the current position ROI reaches 3% and closes the position
func (s *Strategy) maybeRoiTakeProfit(ctx context.Context, latestPrice fixedpoint.Value) (bool, error) {
	if s.Position == nil || s.OrderExecutor == nil {
		return false, nil
	}

	if s.Position.GetBase().IsZero() {
		return false, nil
	}

	base := s.Position.GetBase()
	if base.IsZero() || latestPrice.IsZero() || s.Position.AverageCost.IsZero() {
		return false, nil
	}
	roi := s.Position.ROI(latestPrice)
	threshold := s.TakeProfitROI
	if threshold.IsZero() {
		threshold = fixedpoint.NewFromFloat(0.03)
	}
	if roi.Compare(threshold) >= 0 {
		thresholdStr := threshold.FormatPercentage(2)
		roiStr := roi.FormatPercentage(2)
		s.logger.Infof(
			"take-profit triggered: ROI=%s (threshold=%s), avgCost=%s lastPrice=%s — closing position",
			roiStr,
			thresholdStr,
			s.Position.AverageCost.String(),
			latestPrice.String(),
		)

		_ = s.OrderExecutor.GracefulCancel(ctx)

		if err := s.OrderExecutor.ClosePosition(ctx, fixedpoint.One); err != nil {
			return false, err
		}

		return true, nil
	}
	return false, nil
}

// isBearishEngulfing checks if current (c) kline forms a bearish engulfing over previous (p)
// Conditions:
// 1) c is bearish (c.Close < c.Open)
// 2) Body(c) >= Body(p) * cfg.BodyMultiple (if BodyMultiple == 0, skip this check)
// 3) c.Close < p.Low (close below previous low)
// 4) bottom shadow ratio of c <= cfg.BottomShadowMaxRatio (if > 0)
func (s *Strategy) isBearishEngulfing(p, c types.KLine, cfg *EngulfingTakeProfitConfig) bool {
	if cfg == nil || !cfg.Enabled {
		return false
	}
	// ensure interval match if provided
	if cfg.Interval != "" && c.Interval != cfg.Interval {
		return false
	}

	if !(c.GetClose().Compare(c.GetOpen()) < 0) {
		return false
	}

	bodyPrev := p.GetClose().Sub(p.GetOpen()).Abs()
	bodyCurr := c.GetOpen().Sub(c.GetClose()).Abs()
	if cfg.BodyMultiple.Sign() > 0 {
		if bodyPrev.Sign() == 0 {
			return false
		}
		req := bodyPrev.Mul(cfg.BodyMultiple)
		if bodyCurr.Compare(req) < 0 {
			return false
		}
	}
	if !(c.GetClose().Compare(p.GetLow()) < 0) {
		return false
	}

	if cfg.BottomShadowMaxRatio.Sign() > 0 {
		// bottom shadow ratio ~ (close - low) / close for bearish bar
		den := c.GetClose()
		if den.Sign() == 0 {
			return false
		}
		shadow := c.GetClose().Sub(c.GetLow()).Div(den)
		if shadow.Compare(cfg.BottomShadowMaxRatio) > 0 {
			return false
		}
	}

	return true
}

// isBullishEngulfing checks if current (c) kline forms a bullish engulfing over previous (p)
// Conditions:
// 1) c is bullish (c.Close > c.Open)
// 2) Body(c) >= Body(p) * cfg.BodyMultiple (if BodyMultiple == 0, skip this check)
// 3) c.Close > p.High (close above previous high)
// 4) optional upper shadow ratio check using cfg.BottomShadowMaxRatio as cap (if > 0)
func (s *Strategy) isBullishEngulfing(p, c types.KLine, cfg *EngulfingTakeProfitConfig) bool {
	if cfg == nil || !cfg.Enabled {
		return false
	}
	if cfg.Interval != "" && c.Interval != cfg.Interval {
		return false
	}

	if !(c.GetClose().Compare(c.GetOpen()) > 0) {
		return false
	}

	bodyPrev := p.GetClose().Sub(p.GetOpen()).Abs()
	bodyCurr := c.GetClose().Sub(c.GetOpen()).Abs()
	if cfg.BodyMultiple.Sign() > 0 {
		if bodyPrev.Sign() == 0 {
			return false
		}
		req := bodyPrev.Mul(cfg.BodyMultiple)
		if bodyCurr.Compare(req) < 0 {
			return false
		}
	}
	if !(c.GetClose().Compare(p.GetHigh()) > 0) {
		return false
	}

	if cfg.BottomShadowMaxRatio.Sign() > 0 {
		// use it as an upper shadow cap for bullish bar: (high - close) / close
		den := c.GetClose()
		if den.Sign() == 0 {
			return false
		}
		shadow := c.GetHigh().Sub(c.GetClose()).Div(den)
		if shadow.Compare(cfg.BottomShadowMaxRatio) > 0 {
			return false
		}
	}

	return true
}

// maybeEngulfingTakeProfit evaluates the last two klines and closes position if profitable
func (s *Strategy) maybeEngulfingTakeProfit(ctx context.Context, k types.KLine) {
	if s.EngulfingTakeProfit == nil || !s.EngulfingTakeProfit.Enabled {
		return
	}

	if s.Position == nil || s.OrderExecutor == nil || s.Position.GetBase().IsZero() {
		return
	}

	// Only evaluate on configured interval's close
	interval := s.EngulfingTakeProfit.Interval
	if k.Interval != interval || !k.Closed {
		return
	}

	// query last two klines ending at k.EndTime
	end := k.EndTime.Time()
	kLines, err := s.premiumSession.Exchange.QueryKLines(ctx, s.PremiumSymbol, interval, types.KLineQueryOptions{EndTime: &end, Limit: 2})
	if err != nil || len(kLines) < 2 {
		return
	}

	prev := kLines[0]
	curr := kLines[1]
	if !curr.Closed {
		return
	}

	matched := false
	pattern := ""
	if s.Position.IsLong() {
		matched = s.isBearishEngulfing(prev, curr, s.EngulfingTakeProfit)
		pattern = "bearish engulfing"
	} else if s.Position.IsShort() {
		matched = s.isBullishEngulfing(prev, curr, s.EngulfingTakeProfit)
		pattern = "bullish engulfing"
	}

	if matched {
		latest := curr.GetClose()
		if latest.IsZero() {
			return
		}

		roi := s.Position.ROI(latest)
		if roi.Sign() > 0 {
			s.logger.Infof("%s (%s) detected, ROI %s > 0, closing position to take profit", pattern, interval, roi.FormatPercentage(2))
			_ = s.OrderExecutor.GracefulCancel(ctx)
			_ = s.OrderExecutor.ClosePosition(ctx, fixedpoint.One, pattern)
		}
	}
}

func (s *Strategy) premiumWorker(ctx context.Context) {

	var logLimiter = rate.NewLimiter(rate.Every(1*time.Second), 1)
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("context canceled, stop premium worker")
			return
		case <-s.premiumBook.C:
			// fallthrough to evaluate when either book updates
		case <-s.baseBook.C:
		}

		bookA := s.premiumBook
		bookB := s.baseBook
		if ok, err := bookA.IsValid(); !ok || err != nil {
			continue
		}

		if ok, err := bookB.IsValid(); !ok || err != nil {
			continue
		}

		premium, discount, bidA, askA, bidB, askB, ok := s.compareBooks()
		if !ok {
			continue
		}

		if logLimiter.Allow() {
			s.logger.Infof("comparing books: premium bid=%s ask=%s | base bid=%s ask=%s => premium=%.4f%% discount=%.4f%%",
				bidA.Price.String(), askA.Price.String(), bidB.Price.String(), askB.Price.String(),
				premium*100, discount*100)
		}

		side := s.decideSignal(premium, discount)
		if side == "" {
			continue
		}

		s.logger.Infof(
			"xpremium signal: %s premium=%.4f%% discount=%.4f%% minSpread=%.4f%% pBid=%s pAsk=%s bBid=%s bAsk=%s",
			side.String(), premium*100, discount*100, s.MinSpread.Float64()*100,
			bidA.Price.String(), askA.Price.String(), bidB.Price.String(), askB.Price.String(),
		)

		bbgo.Notify(&Signal{
			Side:       side,
			Premium:    fixedpoint.NewFromFloat(premium),
			Discount:   fixedpoint.NewFromFloat(discount),
			MinSpread:  s.MinSpread,
			PremiumBid: bidA.Price,
			PremiumAsk: askA.Price,
			BaseBid:    bidB.Price,
			BaseAsk:    askB.Price,
			Symbol:     s.TradingSymbol,
		})

		// simple position alignment: avoid re-entering same direction immediately
		if (side == types.SideTypeBuy && s.Position != nil && s.Position.IsLong()) ||
			(side == types.SideTypeSell && s.Position != nil && s.Position.IsShort()) {
			continue
		}

		if err := s.executeSignal(ctx, side, time.Now()); err != nil {
			s.logger.WithError(err).Error("executeSignal error")
		}
	}
}

// loadBacktestCSV loads bid/ask CSV and stores by minute-precision time
func (s *Strategy) loadBacktestCSV(path string) error {
	if path == "" {
		return fmt.Errorf("backtest.bidAskPriceCsv is empty")
	}

	s.logger.Infof("loading backtest bid/ask CSV: %s", path)

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true

	records, err := r.ReadAll()
	if err != nil {
		return err
	}

	if len(records) < 2 {
		return fmt.Errorf("no data in csv: %s", path)
	}

	s.logger.Infof("%d records loaded from backtest CSV", len(records)-1)

	// initialize map
	if s.btData == nil {
		s.btData = make(map[time.Time]backtestBidAsk, len(records)-1)
	}

	// skip header (records[0])
	for i := 1; i < len(records); i++ {
		row := records[i]
		if len(row) < 5 {
			continue
		}

		// parse time in local timezone
		t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(row[0]), time.Local)
		if err != nil {
			return fmt.Errorf("parse time at row %d: %w", i+1, err)
		}

		// columns: 2 base ask, 3 base bid, 4 premium ask, 5 premium bid
		ba := strings.TrimSpace(row[1])
		bb := strings.TrimSpace(row[2])
		pa := strings.TrimSpace(row[3])
		pb := strings.TrimSpace(row[4])

		pAsk, err := parseNum(pa)
		if err != nil {
			return fmt.Errorf("parse premium ask at row %d: %w", i+1, err)
		}

		pBid, err := parseNum(pb)
		if err != nil {
			return fmt.Errorf("parse premium bid at row %d: %w", i+1, err)
		}

		bAsk, err := parseNum(ba)
		if err != nil {
			return fmt.Errorf("parse base ask at row %d: %w", i+1, err)
		}

		bBid, err := parseNum(bb)
		if err != nil {
			return fmt.Errorf("parse base bid at row %d: %w", i+1, err)
		}

		key := t.Truncate(time.Minute).UTC()
		data := backtestBidAsk{time: key, baseAsk: bAsk, baseBid: bBid, premiumAsk: pAsk, premiumBid: pBid}

		// s.logger.Infof("loaded backtest bid/ask at %s: %s, %s, %s, %s", key, ba, bb, pa, pb)
		s.btData[key] = data
	}

	s.logger.Infof("%d records loaded and parsed from backtest CSV", len(s.btData))
	return nil
}

func (s *Strategy) lookupBacktestAt(t time.Time) (backtestBidAsk, bool) {
	if s.btData == nil {
		return backtestBidAsk{}, false
	}

	key := t.Truncate(time.Minute)
	if v, ok := s.btData[key]; ok {
		return v, true
	}

	return backtestBidAsk{}, false
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.PremiumSymbol, types.SubscribeOptions{Interval: "1m"})
	session.Subscribe(types.KLineChannel, s.PremiumSymbol, types.SubscribeOptions{Interval: "15m"})
	session.Subscribe(types.KLineChannel, s.PremiumSymbol, types.SubscribeOptions{Interval: "1h"})
	session.Subscribe(types.KLineChannel, s.PremiumSymbol, types.SubscribeOptions{Interval: "1d"})

	// subscribe klines for engulfing take-profit detection if enabled
	if s.EngulfingTakeProfit != nil && s.EngulfingTakeProfit.Enabled {
		interval := s.EngulfingTakeProfit.Interval
		if interval == "" {
			interval = types.Interval1h
		}

		session.Subscribe(types.KLineChannel, s.PremiumSymbol, types.SubscribeOptions{Interval: interval})
	}
}

// Run is only used for back-testing with single session
func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// in backtest, we run on a single session; use premium as trading session
	if !bbgo.IsBackTesting {
		return nil
	}

	// override the settings
	s.BaseSession = s.PremiumSession
	s.TradingSession = s.PremiumSession

	s.premiumSession = session
	s.baseSession = session
	s.tradingSession = session

	if s.Symbol == "" {
		s.Symbol = s.TradingSymbol
	}

	// market and common strategy init
	market, ok := session.Market(s.TradingSymbol)
	if !ok {
		return fmt.Errorf("market %s not found in backtest session", s.TradingSymbol)
	}

	s.tradingMarket = market
	s.Strategy.Initialize(ctx, s.Environment, session, market, ID, s.InstanceID())

	s.pvLow = indicatorv2.PivotLow(s.premiumSession.Indicators(s.PremiumSymbol).LOW(s.PivotStop.Interval), s.PivotStop.Left, s.PivotStop.Right)
	s.pvHigh = indicatorv2.PivotHigh(s.premiumSession.Indicators(s.PremiumSymbol).HIGH(s.PivotStop.Interval), s.PivotStop.Left, s.PivotStop.Right)
	s.kLines = s.premiumSession.Indicators(s.PremiumSymbol).KLines(s.PivotStop.Interval)

	// load csv if configured
	if s.BacktestConfig == nil || s.BacktestConfig.BidAskPriceCsv == "" {
		return fmt.Errorf("backtest config or csv path not provided: %+v", s.BacktestConfig)
	}

	if err := s.loadBacktestCSV(s.BacktestConfig.BidAskPriceCsv); err != nil {
		return err
	}

	tradingInterval := types.Interval1h
	if s.BacktestConfig.TradingInterval != "" {
		tradingInterval = s.BacktestConfig.TradingInterval
	}

	if s.EngulfingTakeProfit != nil && s.EngulfingTakeProfit.Enabled {
		session.MarketDataStream.OnKLineClosed(types.KLineWith(s.PremiumSymbol, s.EngulfingTakeProfit.Interval, func(k types.KLine) {
			// engulfing take-profit check triggered by kline close events as well
			s.maybeEngulfingTakeProfit(ctx, k)
		}))
	}

	session.MarketDataStream.OnKLine(types.KLineWith(s.PremiumSymbol, types.Interval1m, func(k types.KLine) {
		// backtest ROI take-profit using kline close
		if _, err := s.maybeRoiTakeProfit(ctx, k.GetClose()); err != nil {
			s.logger.WithError(err).Warn("backtest take-profit error")
		}
	}))

	// subscribe to klines for time alignment; assume 1m unless different backtest interval
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.PremiumSymbol, tradingInterval, func(k types.KLine) {
		// match kline time with csv time; prefer k.EndTime
		rec, ok := s.lookupBacktestAt(k.EndTime.Time().Add(time.Millisecond))
		if !ok {
			return
		}

		// rebuild best bid/ask as PriceVolume
		pBid := types.PriceVolume{Price: rec.premiumBid, Volume: fixedpoint.One}
		pAsk := types.PriceVolume{Price: rec.premiumAsk, Volume: fixedpoint.One}
		bBid := types.PriceVolume{Price: rec.baseBid, Volume: fixedpoint.One}
		bAsk := types.PriceVolume{Price: rec.baseAsk, Volume: fixedpoint.One}

		premium, discount := s.computeSpreads(pBid, pAsk, bBid, bAsk)
		side := s.decideSignal(premium, discount)
		if side == "" {
			return
		}

		// synchronous execution in backtest
		if err := s.executeSignal(ctx, side, k.EndTime.Time()); err != nil {
			s.logger.WithError(err).Error("backtest executeSignal error")
		}
	}))

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		_ = s.OrderExecutor.GracefulCancel(ctx)

		_, _ = fmt.Fprintln(os.Stderr, s.TradeStats.String())
	})

	return nil
}

func parseNum(sv string) (fixedpoint.Value, error) {
	sv = strings.TrimSpace(sv)

	fv, err := fixedpoint.NewFromString(sv)
	if err == nil {
		return fv, nil
	}

	f, err := strconv.ParseFloat(sv, 64)
	if err != nil {
		return fixedpoint.Zero, err
	}

	return fixedpoint.NewFromFloat(f), nil
}

func (s *Strategy) syncPositionRisks(ctx context.Context, riskService types.ExchangeRiskService, symbol string) error {
	positionRisks, err := riskService.QueryPositionRisk(ctx)
	if err != nil {
		return err
	}

	s.logger.Infof("fetched futures position risks: %+v", positionRisks)

	if len(positionRisks) == 0 {
		s.Position.Reset()
		return nil
	}

	for _, positionRisk := range positionRisks {
		if positionRisk.Symbol != symbol {
			continue
		}

		if positionRisk.PositionAmount.IsZero() || positionRisk.EntryPrice.IsZero() {
			continue
		}

		s.Position.Base = positionRisk.PositionAmount
		s.Position.AverageCost = positionRisk.EntryPrice
		s.logger.Infof("restored futures position from positionRisk: base=%s, average_cost=%s, position_risk=%+v",
			s.Position.Base.String(),
			s.Position.AverageCost.String(),
			positionRisk)
	}

	return nil
}
