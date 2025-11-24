package xpremium

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
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

	// MinTriggers is the minimum number of consecutive premium/discount triggers required
	// before opening a position. A value of 0 opens immediately on first trigger. Default is 3.
	MinTriggers int `json:"minTriggers"`

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

	// QuoteDepth is the quoting depth in quote currency (e.g., USDT) when deriving bid/ask from depth books
	// Defaults to 300 USDT if not specified.
	QuoteDepth fixedpoint.Value `json:"quoteDepth"`

	logger        logrus.FieldLogger
	metricsLabels prometheus.Labels

	// cached metrics to avoid repeated With() lookups
	premiumRatioObs  prometheus.Observer
	discountRatioObs prometheus.Observer
	sigCounterBuy    prometheus.Counter
	sigCounterSell   prometheus.Counter
	sigRatioObsBuy   prometheus.Observer
	sigRatioObsSell  prometheus.Observer

	premiumSession, baseSession, tradingSession *bbgo.ExchangeSession
	tradingMarket                               types.Market

	// runtime fields
	premiumBook *types.StreamOrderBook
	baseBook    *types.StreamOrderBook
	// depth books derived from stream books
	premiumDepthBook *types.DepthBook
	baseDepthBook    *types.DepthBook

	premiumStream types.Stream
	baseStream    types.Stream

	// add connector manager to manage connectors/streams
	connectorManager *types.ConnectorManager

	// backtest data map keyed by minute-precision time
	btData map[time.Time]backtestBidAsk

	pvHigh *indicatorv2.PivotHighStream
	pvLow  *indicatorv2.PivotLowStream
	kLines *indicatorv2.KLineStream

	// trigger counters for consecutive signals
	buyTriggerCount  int
	sellTriggerCount int

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

		"base_session":    s.BaseSession,
		"premium_session": s.PremiumSession,
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

	// cache metrics objects to avoid repeated With() lookups
	base := s.metricsLabels
	if base != nil {
		// histograms without side label
		s.premiumRatioObs = premiumRatioHistogram.With(base)
		s.discountRatioObs = discountRatioHistogram.With(base)

		// signal metrics (with side)
		curriedSigCounter := signalCounter.MustCurryWith(base)
		curriedSigHist := signalRatioHistogram.MustCurryWith(base)
		s.sigCounterBuy = curriedSigCounter.With(prometheus.Labels{"side": types.SideTypeBuy.String()})
		s.sigCounterSell = curriedSigCounter.With(prometheus.Labels{"side": types.SideTypeSell.String()})
		s.sigRatioObsBuy = curriedSigHist.With(prometheus.Labels{"side": types.SideTypeBuy.String()})
		s.sigRatioObsSell = curriedSigHist.With(prometheus.Labels{"side": types.SideTypeSell.String()})
	}
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

	// default depth in quote to 300 USDT
	if s.QuoteDepth.IsZero() {
		s.QuoteDepth = fixedpoint.NewFromInt(300)
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

	// default minTriggers to 3 if not set
	if s.MinTriggers == 0 {
		s.MinTriggers = 3
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
	if s.QuoteDepth.Sign() <= 0 {
		return fmt.Errorf("quoteDepth must be > 0 (in quote currency, e.g. 300 for 300 USDT)")
	}
	if s.MinTriggers < 0 {
		return fmt.Errorf("minTriggers must be >= 0")
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
		s.logger.Warnf("backtesting mode detected, xpremium strategy is not supported in backtesting")
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

		if !s.Position.GetBase().IsZero() {
			bbgo.Notify(s.Position)
		}
	}

	if err := s.loadOpenOrders(ctx); err != nil {
		return err
	}

	// register engulfing take-profit handler on kline close
	if s.EngulfingTakeProfit != nil && s.EngulfingTakeProfit.Enabled {
		interval := s.EngulfingTakeProfit.Interval
		s.premiumSession.MarketDataStream.OnKLineClosed(types.KLineWith(s.PremiumSymbol, interval, func(k types.KLine) {
			s.maybeEngulfingTakeProfit(ctx, k)
		}))
	}

	s.tradingSession.MarketDataStream.OnKLineClosed(types.KLineWith(s.TradingSymbol, types.Interval1m, func(k types.KLine) {
		// update ROI take-profit and trailing stop on 1m kline close
		if _, err := s.maybeRoiTakeProfit(ctx, k.GetClose()); err != nil {
			s.logger.WithError(err).Warn("take-profit error")
		}

		if err := s.maybeUpdateTrailingStop(ctx, k.GetClose()); err != nil {
			s.logger.WithError(err).Warn("trailing stop update error")
		}
	}))

	// allocate isolated public streams for books and bind StreamBooks
	premiumStream := bbgo.NewBookStream(s.premiumSession, s.PremiumSymbol, types.SubscribeOptions{
		Depth: types.DepthLevelFull,
		Speed: types.SpeedHigh,
	})
	baseStream := bbgo.NewBookStream(s.baseSession, s.BaseSymbol)

	s.premiumStream, s.baseStream = premiumStream, baseStream

	s.premiumBook = types.NewStreamBook(s.PremiumSymbol, s.premiumSession.ExchangeName)
	s.premiumBook.BindStream(premiumStream)
	s.premiumDepthBook = types.NewDepthBook(s.premiumBook)

	s.baseBook = types.NewStreamBook(s.BaseSymbol, s.baseSession.ExchangeName)
	s.baseBook.BindStream(baseStream)
	s.baseDepthBook = types.NewDepthBook(s.baseBook)

	// register streams into the connector manager and connect them via connector manager
	s.connectorManager.Add(premiumStream, baseStream)

	if err := s.connectorManager.Connect(ctx); err != nil {
		s.logger.WithError(err).Error("connector manager connect error")
		return err
	}

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		bbgo.Sync(ctx, s)

		_, _ = fmt.Fprintln(os.Stderr, s.TradeStats.String())
	})

	go func() {
		s.logger.Infof("waiting for authentication of premium and base sessions...")
		select {
		case <-ctx.Done():
			return
		case <-s.premiumSession.UserDataConnectivity.AuthedC():
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

// compareBooksAtQuoteDepth fetches bid/ask using depth-averaged prices at the given quote depth
func (s *Strategy) compareBooksAtQuoteDepth(depth fixedpoint.Value) (premium, discount float64, pBid, pAsk, bBid, bAsk types.PriceVolume, ok bool) {
	if s.premiumDepthBook == nil || s.baseDepthBook == nil {
		return 0, 0, types.PriceVolume{}, types.PriceVolume{}, types.PriceVolume{}, types.PriceVolume{}, false
	}

	// compute depth prices
	pBidPrice, pAskPrice := s.premiumDepthBook.BestBidAndAskAtQuoteDepth(depth)
	bBidPrice, bAskPrice := s.baseDepthBook.BestBidAndAskAtQuoteDepth(depth)
	if pBidPrice.IsZero() || pAskPrice.IsZero() || bBidPrice.IsZero() || bAskPrice.IsZero() {
		return 0, 0, types.PriceVolume{}, types.PriceVolume{}, types.PriceVolume{}, types.PriceVolume{}, false
	}

	// Wrap into PriceVolume with unit volumes (volume not used in spread calc)
	pBid = types.PriceVolume{Price: pBidPrice, Volume: fixedpoint.One}
	pAsk = types.PriceVolume{Price: pAskPrice, Volume: fixedpoint.One}
	bBid = types.PriceVolume{Price: bBidPrice, Volume: fixedpoint.One}
	bAsk = types.PriceVolume{Price: bAskPrice, Volume: fixedpoint.One}

	prem, disc := s.computeSpreads(pBid, pAsk, bBid, bAsk)
	return prem, disc, pBid, pAsk, bBid, bAsk, true
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

// computeTrailingStopTarget computes the desired trailing stop based on the latest price
// and current position side. For long: price * (1 - 0.002); for short: price * (1 + 0.002).
// Returns zero value and false if position side is unknown.
func (s *Strategy) computeTrailingStopTarget(latestPrice fixedpoint.Value) (fixedpoint.Value, bool) {
	if s.Position == nil || s.Position.GetBase().IsZero() || latestPrice.IsZero() {
		return fixedpoint.Zero, false
	}
	if s.Position.IsLong() {
		return latestPrice.Mul(fixedpoint.NewFromFloat(1.0 - 0.002)), true
	}
	if s.Position.IsShort() {
		return latestPrice.Mul(fixedpoint.NewFromFloat(1.0 + 0.002)), true
	}
	return fixedpoint.Zero, false
}

// roundStopToTick applies tick-size rounding for stop prices.
// For long (stop below market), rounding up would loosen, so we round down (floor).
// For short (stop above market), rounding down would loosen, so we round up (ceil).
func (s *Strategy) roundStopToTick(side types.SideType, price fixedpoint.Value) fixedpoint.Value {
	tick := s.tradingMarket.TickSize
	if tick.Sign() <= 0 || price.IsZero() {
		return price
	}
	f := price.Div(tick)
	switch side {
	case types.SideTypeBuy:
		q := math.Floor(f.Float64())
		return tick.Mul(fixedpoint.NewFromFloat(q))
	case types.SideTypeSell:
		q := math.Ceil(f.Float64())
		return tick.Mul(fixedpoint.NewFromFloat(q))
	default:
		return price
	}
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

	// 1) Pivot-based with distance check; fallback to recent 15m klines if too far (>2%)
	if pivot := s.findPivotStop(side); pivot.Sign() > 0 {
		if ok, _ := s.validateStopPrice(side, currentPrice, pivot); ok {
			// compute distance ratio between current price and pivot stop
			var dist fixedpoint.Value
			if side == types.SideTypeBuy {
				dist = currentPrice.Sub(pivot).Div(currentPrice)
			} else {
				dist = pivot.Sub(currentPrice).Div(currentPrice)
			}
			// threshold 2%
			maxDist := fixedpoint.NewFromFloat(0.02)
			if dist.Sign() <= 0 || dist.Compare(maxDist) <= 0 {
				// within acceptable distance, use pivot
				return pivot, nil
			}
			// too far: try nearest kline stop (recent 15m klines)
			if sp, err := s.findNearestStop(ctx, side, now, 3); err == nil && sp.Sign() > 0 {
				if ok, _ := s.validateStopPrice(side, currentPrice, sp); ok {
					if s.logger != nil {
						s.logger.WithFields(logrus.Fields{"pivot": pivot.String(), "nearest": sp.String(), "dist": dist.String()}).Info("pivot stop too far, using recent kline stop instead")
					}
					return sp, nil
				}
			}
			// fallback to pivot if nearest not available/valid
			return pivot, nil
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

	if qty.Compare(s.tradingMarket.MinQuantity) < 0 {
		qty = s.tradingMarket.MinQuantity
	}

	qty = s.tradingMarket.AdjustQuantityByMinNotional(qty, currentPrice)

	return s.tradingMarket.TruncateQuantity(qty), nil
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
		s.logger.WithError(qerr).Warnf("position sizing failed, fallback to min qty %s", s.tradingMarket.MinQuantity.String())
		qty = s.tradingMarket.MinQuantity
	}

	qty = s.tradingMarket.TruncateQuantity(qty)

	submitOrder := types.SubmitOrder{
		Symbol:   s.TradingSymbol,
		Side:     side,
		Type:     types.OrderTypeMarket,
		Quantity: qty,
		Market:   s.tradingMarket,

		Tag: "entry",
	}

	s.logger.Infof("submitting %s order: %+v", s.TradingSymbol, submitOrder)

	_, err = s.OrderExecutor.SubmitOrders(ctx, submitOrder)
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
// maybeUpdateTrailingStop adjusts existing stop-loss to trail when ROI exceeds threshold
// If ROI > 1%, move stop to 0.2% away from current price in the profitable direction:
// - Long: set stop at price * (1 - 0.002)
// - Short: set stop at price * (1 + 0.002)
// Only tightens the stop (never loosens) and respects market tick size.
func (s *Strategy) maybeUpdateTrailingStop(ctx context.Context, latestPrice fixedpoint.Value) error {
	if s.Position == nil || s.OrderExecutor == nil {
		return nil
	}
	if s.Position.GetBase().IsZero() || latestPrice.IsZero() || s.Position.AverageCost.IsZero() {
		return nil
	}

	roi := s.Position.ROI(latestPrice)
	// threshold = 1%
	if roi.Compare(fixedpoint.NewFromFloat(0.01)) < 0 {
		return nil
	}

	// find existing stop-loss order tagged by "stop-loss"
	active := s.OrderExecutor.ActiveMakerOrders()
	var stopOrder *types.Order
	for _, o := range active.Orders() {
		if o.Symbol != s.TradingSymbol {
			continue
		}
		if o.Type != types.OrderTypeStopMarket {
			continue
		}

		// Prefer the first matched; if multiple exist, we'll work on the first
		ord := o
		stopOrder = &ord
		break
	}

	if stopOrder == nil {
		return nil
	}

	// compute target new stop via helper
	target, ok := s.computeTrailingStopTarget(latestPrice)
	if !ok {
		return nil
	}

	// determine side for validations and rounding
	side := types.SideTypeBuy
	if s.Position.IsShort() {
		side = types.SideTypeSell
	}

	// apply tick rounding via helper
	target = s.roundStopToTick(side, target)

	// validate stop against current price and side
	if ok, err := s.validateStopPrice(side, latestPrice, target); !ok {
		if err != nil {
			s.logger.WithError(err).Debug("trailing stop validation failed")
		}

		return nil
	}

	// ensure target tightens compared to existing stop
	existing := stopOrder.StopPrice
	if existing.IsZero() {
		// if exchange didn't return stop price in order, proceed
	} else {
		if s.Position.IsLong() {
			// only raise the stop
			if target.Compare(existing) <= 0 {
				return nil
			}
		} else {
			// only lower the stop for shorts
			if target.Compare(existing) >= 0 {
				return nil
			}
		}
	}

	// ensure at least one tick difference
	tick := s.tradingMarket.TickSize
	if tick.Sign() > 0 && target.Sub(existing).Abs().Compare(tick) < 0 {
		return nil
	}

	// cancel existing stop and submit a replacement
	if err := s.OrderExecutor.GracefulCancel(ctx, *stopOrder); err != nil {
		s.logger.WithError(err).Warn("failed to cancel existing stop-loss for trailing update")
		return err
	}

	qty := stopOrder.Quantity
	if qty.IsZero() {
		// fallback to absolute base position size
		qty = s.Position.GetBase().Abs()
	}

	newStop := types.SubmitOrder{
		Market:        s.tradingMarket,
		Symbol:        s.TradingSymbol,
		Side:          stopOrder.Side,
		Type:          types.OrderTypeStopMarket,
		Quantity:      qty,
		StopPrice:     target,
		ClosePosition: true,
		Tag:           "stop-loss",
	}
	if _, err := s.OrderExecutor.SubmitOrders(ctx, newStop); err != nil {
		s.logger.WithError(err).Warnf("submit trailing stop-loss failed: %+v", newStop)
		return err
	}

	s.logger.Infof("updated trailing stop to %s (roi=%s)", target.String(), roi.FormatPercentage(2))
	return nil
}

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
		}

		bookA := s.premiumBook
		bookB := s.baseBook
		if ok, err := bookA.IsValid(); !ok || err != nil {
			continue
		}

		if ok, err := bookB.IsValid(); !ok || err != nil {
			continue
		}

		// Use depth-averaged prices at configured quote depth for comparisons
		premium, discount, bidA, askA, bidB, askB, ok := s.compareBooksAtQuoteDepth(s.QuoteDepth)
		if !ok {
			continue
		}

		// observe comparison metrics (cached observers)
		if s.premiumRatioObs != nil {
			s.premiumRatioObs.Observe(math.Abs(premium))
		}
		if s.discountRatioObs != nil {
			s.discountRatioObs.Observe(math.Abs(discount))
		}

		if logLimiter.Allow() {
			s.logger.Infof("comparing books: premium bid=%s ask=%s | base bid=%s ask=%s => premium=%.4f%% discount=%.4f%%",
				bidA.Price.String(), askA.Price.String(), bidB.Price.String(), askB.Price.String(),
				premium*100, discount*100)
		}

		side := s.decideSignal(premium, discount)
		if side == "" {
			// reset counters when condition is not met
			s.buyTriggerCount = 0
			s.sellTriggerCount = 0
			continue
		}

		// signal metrics (cached)
		var sigRatio float64
		if side == types.SideTypeBuy {
			sigRatio = math.Abs(premium)
			if s.sigCounterBuy != nil {
				s.sigCounterBuy.Inc()
			}
			if s.sigRatioObsBuy != nil {
				s.sigRatioObsBuy.Observe(sigRatio)
			}
		} else {
			sigRatio = math.Abs(discount)
			if s.sigCounterSell != nil {
				s.sigCounterSell.Inc()
			}
			if s.sigRatioObsSell != nil {
				s.sigRatioObsSell.Observe(sigRatio)
			}
		}

		// update trigger counters and gate execution until count > MinTriggers
		if side == types.SideTypeBuy {
			s.buyTriggerCount++
			s.sellTriggerCount = 0
		} else if side == types.SideTypeSell {
			s.sellTriggerCount++
			s.buyTriggerCount = 0
		}

		// check gating based on MinTriggers (strictly greater-than as requested)
		if s.MinTriggers > 0 {
			if (side == types.SideTypeBuy && s.buyTriggerCount <= s.MinTriggers) ||
				(side == types.SideTypeSell && s.sellTriggerCount <= s.MinTriggers) {
				// not enough consecutive triggers yet
				continue
			}
		}

		s.logger.Infof(
			"xpremium signal: %s premium=%.4f%% discount=%.4f%% minSpread=%.4f%% minTriggers=%d trigCount(buy=%d,sell=%d) pBid=%s pAsk=%s bBid=%s bAsk=%s",
			side.String(), premium*100, discount*100, s.MinSpread.Float64()*100, s.MinTriggers, s.buyTriggerCount, s.sellTriggerCount,
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
		} else {
			// reset counters after entering a position
			s.buyTriggerCount = 0
			s.sellTriggerCount = 0
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

	// subscribe trading symbol 1m klines for backtest TP/TS updates
	session.Subscribe(types.KLineChannel, s.TradingSymbol, types.SubscribeOptions{Interval: types.Interval1m})

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

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.TradingSymbol, types.Interval1m, func(k types.KLine) {
		// backtest ROI take-profit using kline close
		if _, err := s.maybeRoiTakeProfit(ctx, k.GetClose()); err != nil {
			s.logger.WithError(err).Warn("backtest take-profit error")
		}
		// also update trailing stop with the latest close price
		if err := s.maybeUpdateTrailingStop(ctx, k.GetClose()); err != nil {
			s.logger.WithError(err).Warn("backtest trailing stop update error")
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

		// observe comparison metrics in backtest (cached)
		if s.premiumRatioObs != nil {
			s.premiumRatioObs.Observe(math.Abs(premium))
		}
		if s.discountRatioObs != nil {
			s.discountRatioObs.Observe(math.Abs(discount))
		}

		side := s.decideSignal(premium, discount)
		if side == "" {
			// reset counters when condition is not met
			s.buyTriggerCount = 0
			s.sellTriggerCount = 0
			return
		}

		// update trigger counters
		if side == types.SideTypeBuy {
			s.buyTriggerCount++
			s.sellTriggerCount = 0
		} else if side == types.SideTypeSell {
			s.sellTriggerCount++
			s.buyTriggerCount = 0
		}

		// gate by MinTriggers (> MinTriggers)
		if s.MinTriggers > 0 {
			if (side == types.SideTypeBuy && s.buyTriggerCount <= s.MinTriggers) ||
				(side == types.SideTypeSell && s.sellTriggerCount <= s.MinTriggers) {
				return
			}
		}

		// signal metrics in backtest (cached)
		if side == types.SideTypeBuy {
			if s.sigCounterBuy != nil {
				s.sigCounterBuy.Inc()
			}
			if s.sigRatioObsBuy != nil {
				s.sigRatioObsBuy.Observe(math.Abs(premium))
			}
		} else {
			if s.sigCounterSell != nil {
				s.sigCounterSell.Inc()
			}
			if s.sigRatioObsSell != nil {
				s.sigRatioObsSell.Observe(math.Abs(discount))
			}
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

func (s *Strategy) loadOpenOrders(ctx context.Context) error {
	openOrders, err := s.tradingSession.Exchange.QueryOpenOrders(ctx, s.TradingSymbol)
	if err != nil {
		return fmt.Errorf("unable to query open orders, error: %w", err)
	}

	activeBook := s.OrderExecutor.ActiveMakerOrders()
	for _, order := range openOrders {
		activeBook.Add(order)
	}

	return nil
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
