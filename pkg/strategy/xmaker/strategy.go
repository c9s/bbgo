package xmaker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/dynamic"
	"github.com/c9s/bbgo/pkg/exchange/sandbox"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/pricesolver"
	"github.com/c9s/bbgo/pkg/profile/timeprofile"
	"github.com/c9s/bbgo/pkg/risk/circuitbreaker"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/strategy/xmaker/pricer"
	"github.com/c9s/bbgo/pkg/strategy/xmaker/signal"
	"github.com/c9s/bbgo/pkg/style"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/c9s/bbgo/pkg/util/timejitter"
	"github.com/c9s/bbgo/pkg/util/tradingutil"
)

var defaultMargin = fixedpoint.NewFromFloat(0.003)
var two = fixedpoint.NewFromInt(2)

const feeTokenQuote = "USDT"

const priceUpdateTimeout = 30 * time.Second

const ID = "xmaker"

var log = logrus.WithField("strategy", ID)

func nopCover(v fixedpoint.Value) {}

type MutexFloat64 struct {
	value float64
	mu    sync.Mutex
}

func (m *MutexFloat64) Set(v float64) {
	m.mu.Lock()
	m.value = v
	m.mu.Unlock()
}

func (m *MutexFloat64) Get() float64 {
	m.mu.Lock()
	v := m.value
	m.mu.Unlock()
	return v
}

type Quote struct {
	BestBidPrice, BestAskPrice fixedpoint.Value

	BidMargin, AskMargin fixedpoint.Value

	// BidLayerPips is the price pips between each layer
	BidLayerPips, AskLayerPips fixedpoint.Value
}

type SessionBinder interface {
	Bind(ctx context.Context, session *bbgo.ExchangeSession, symbol string) error
}

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type SignalMargin struct {
	Enabled   bool            `json:"enabled"`
	Scale     *bbgo.SlideRule `json:"scale,omitempty"`
	Threshold float64         `json:"threshold,omitempty"`
}

type Strategy struct {
	Environment *bbgo.Environment

	// Symbol is the maker Symbol
	Symbol      string `json:"symbol"`
	MakerSymbol string `json:"makerSymbol"`

	// MakerExchange session name
	MakerExchange string `json:"makerExchange"`
	MakerSession  string `json:"makerSession"`

	// SourceSymbol allows subscribing to a different symbol for price/book
	SourceSymbol string `json:"sourceSymbol,omitempty"`

	// SourceExchange session name
	SourceExchange string `json:"sourceExchange"`
	HedgeSession   string `json:"hedgeSession"`

	UpdateInterval      types.Duration `json:"updateInterval"`
	HedgeInterval       types.Duration `json:"hedgeInterval"`
	OrderCancelWaitTime types.Duration `json:"orderCancelWaitTime"`

	SubscribeFeeTokenMarkets bool `json:"subscribeFeeTokenMarkets"`

	EnableSignalMargin bool `json:"enableSignalMargin"`

	SignalSource string `json:"signalSource,omitempty"`

	SignalConfigList *signal.DynamicConfig `json:"signals"`

	SignalReverseSideMargin       *SignalMargin `json:"signalReverseSideMargin,omitempty"`
	SignalTrendSideMarginDiscount *SignalMargin `json:"signalTrendSideMarginDiscount,omitempty"`

	// Margin is the default margin for the quote
	Margin    fixedpoint.Value `json:"margin"`
	BidMargin fixedpoint.Value `json:"bidMargin"`
	AskMargin fixedpoint.Value `json:"askMargin"`

	// MinMargin is the minimum margin protection for signal margin
	MinMargin *fixedpoint.Value `json:"minMargin"`

	UseDepthPrice bool             `json:"useDepthPrice"`
	DepthQuantity fixedpoint.Value `json:"depthQuantity"`

	// TODO: move SourceDepthLevel to HedgeMarket
	SourceDepthLevel types.Depth `json:"sourceDepthLevel"`
	MakerOnly        bool        `json:"makerOnly"`

	// EnableDelayHedge enables the delay hedge feature
	EnableDelayHedge bool `json:"enableDelayHedge"`
	// MaxHedgeDelayDuration is the maximum delay duration to hedge the position
	MaxDelayHedgeDuration     types.Duration `json:"maxHedgeDelayDuration"`
	DelayHedgeSignalThreshold float64        `json:"delayHedgeSignalThreshold"`

	DelayedHedge *DelayedHedge `json:"delayedHedge,omitempty"`

	SplitHedge *SplitHedge `json:"splitHedge,omitempty"`

	SpreadMaker *SpreadMaker `json:"spreadMaker,omitempty"`

	EnableBollBandMargin bool             `json:"enableBollBandMargin"`
	BollBandInterval     types.Interval   `json:"bollBandInterval"`
	BollBandMargin       fixedpoint.Value `json:"bollBandMargin"`
	BollBandMarginFactor fixedpoint.Value `json:"bollBandMarginFactor"`

	// MinMarginLevel is the minimum margin level to trigger the hedge
	MinMarginLevel fixedpoint.Value `json:"minMarginLevel"`

	StopHedgeQuoteBalance fixedpoint.Value `json:"stopHedgeQuoteBalance"`
	StopHedgeBaseBalance  fixedpoint.Value `json:"stopHedgeBaseBalance"`

	// Quantity is used for fixed quantity of the first layer
	Quantity fixedpoint.Value `json:"quantity"`

	// QuantityMultiplier is the factor that multiplies the quantity of the previous layer
	QuantityMultiplier fixedpoint.Value `json:"quantityMultiplier"`

	// QuantityScale helps user to define the quantity by layer scale
	QuantityScale *bbgo.LayerScale `json:"quantityScale,omitempty"`

	// MaxExposurePosition defines the unhedged quantity of stop
	MaxExposurePosition fixedpoint.Value `json:"maxExposurePosition"`

	MaxHedgeAccountLeverage       fixedpoint.Value `json:"maxHedgeAccountLeverage"`
	MaxHedgeQuoteQuantityPerOrder fixedpoint.Value `json:"maxHedgeQuoteQuantityPerOrder"`

	DisableHedge bool `json:"disableHedge"`

	NotifyTrade                        bool             `json:"notifyTrade"`
	NotifyIgnoreSmallAmountProfitTrade fixedpoint.Value `json:"notifyIgnoreSmallAmountProfitTrade"`

	EnableArbitrage bool `json:"enableArbitrage"`

	// RecoverTrade tries to find the missing trades via the REStful API
	RecoverTrade bool `json:"recoverTrade"`

	RecoverTradeScanPeriod types.Duration `json:"recoverTradeScanPeriod"`

	MaxQuoteQuotaRatio fixedpoint.Value `json:"maxQuoteQuotaRatio,omitempty"`

	NumLayers int `json:"numLayers"`

	// Pips is the pips of the layer prices
	Pips fixedpoint.Value `json:"pips"`

	// ProfitFixerConfig is the profit fixer configuration
	ProfitFixerConfig *common.ProfitFixerConfig `json:"profitFixer,omitempty"`

	UseSandbox              bool                        `json:"useSandbox,omitempty"`
	SandboxExchangeBalances map[string]fixedpoint.Value `json:"sandboxExchangeBalances,omitempty"`

	SyntheticHedge *SyntheticHedge `json:"syntheticHedge,omitempty"`

	// --------------------------------
	// private field

	makerSession, sourceSession *bbgo.ExchangeSession

	makerMarket, sourceMarket types.Market

	// boll is the BOLLINGER indicator we used for predicting the price.
	boll *indicatorv2.BOLLStream

	state *State

	priceSolver    *pricesolver.SimplePriceSolver
	CircuitBreaker *circuitbreaker.BasicCircuitBreaker `json:"circuitBreaker"`

	// persistence fields
	Position    *types.Position `json:"position,omitempty" persistence:"position"`
	ProfitStats *ProfitStats    `json:"profitStats,omitempty" persistence:"profit_stats"`

	tradingCtx    context.Context
	cancelTrading context.CancelFunc

	positionExposure *PositionExposure

	sourceBook, makerBook *types.StreamOrderBook
	depthSourceBook       *types.DepthBook

	activeMakerOrders *bbgo.ActiveOrderBook

	hedgeErrorLimiter         *rate.Limiter
	hedgeErrorRateReservation *rate.Reservation

	orderStore     *core.OrderStore
	tradeCollector *core.TradeCollector

	askPriceHeartBeat, bidPriceHeartBeat *types.PriceHeartBeat

	accountValueCalculator *bbgo.AccountValueCalculator

	lastPrice fixedpoint.MutexValue
	groupID   uint32

	stopQuoteWorkerC chan struct{}
	quoteWorkerDoneC chan struct{}

	wg sync.WaitGroup

	reportProfitStatsRateLimiter *rate.Limiter
	circuitBreakerAlertLimiter   *rate.Limiter

	logger logrus.FieldLogger

	metricsLabels prometheus.Labels

	sourceMarketDataConnectivity, sourceUserDataConnectivity *types.Connectivity
	connectivityGroup                                        *types.ConnectivityGroup

	// lastAggregatedSignal stores the last aggregated signal with mutex
	// TODO: use float64 series instead, so that we can store history signal values
	lastAggregatedSignal MutexFloat64

	positionStartedAt      *time.Time
	positionStartedAtMutex sync.Mutex

	debtQuotaCache *fixedpoint.ExpirableValue

	// metricsCache
	cancelOrderDurationMetrics         prometheus.Observer
	aggregatedSignalMetrics            prometheus.Gauge
	askMarginMetrics, bidMarginMetrics prometheus.Gauge

	makerOrderPlacementDurationMetrics prometheus.Observer
	openOrderBidExposureInUsdMetrics   prometheus.Gauge
	openOrderAskExposureInUsdMetrics   prometheus.Gauge

	simpleHedgeMode bool
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return strings.Join([]string{ID, s.Symbol}, ":")
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	sourceSession, ok := sessions[s.SourceExchange]
	if !ok {
		panic(fmt.Errorf("source session %s is not defined", s.SourceExchange))
	}

	sourceSession.Subscribe(types.KLineChannel, s.SourceSymbol, types.SubscribeOptions{Interval: "1m"})

	makerSession, ok := sessions[s.MakerExchange]
	if !ok {
		panic(fmt.Errorf("maker session %s is not defined", s.MakerExchange))
	}

	makerSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})

	for _, sig := range s.SignalConfigList.Signals {
		sig.Signal.Subscribe(sourceSession, s.SourceSymbol)
	}

	if s.SubscribeFeeTokenMarkets {
		subscribeOpts := types.SubscribeOptions{Interval: "1m"}
		if cu := sourceSession.Exchange.PlatformFeeCurrency(); cu != "" {
			sourceSession.Subscribe(
				types.KLineChannel, sourceSession.Exchange.PlatformFeeCurrency()+feeTokenQuote, subscribeOpts,
			)
		}

		if cu := makerSession.Exchange.PlatformFeeCurrency(); cu != "" {
			makerSession.Subscribe(
				types.KLineChannel, makerSession.Exchange.PlatformFeeCurrency()+feeTokenQuote, subscribeOpts,
			)
		}
	}
}

func aggregatePrice(pvs types.PriceVolumeSlice, requiredQuantity fixedpoint.Value) (price fixedpoint.Value) {
	if len(pvs) == 0 {
		price = fixedpoint.Zero
		return price
	}

	sumAmount := fixedpoint.Zero
	sumQty := fixedpoint.Zero
	for i := 0; i < len(pvs); i++ {
		pv := pvs[i]
		sumQty = sumQty.Add(pv.Volume)
		sumAmount = sumAmount.Add(pv.Volume.Mul(pv.Price))
		if sumQty.Compare(requiredQuantity) >= 0 {
			break
		}
	}

	return sumAmount.Div(sumQty)
}

func (s *Strategy) Initialize() error {
	s.stopQuoteWorkerC = make(chan struct{})
	s.quoteWorkerDoneC = make(chan struct{})

	s.bidPriceHeartBeat = types.NewPriceHeartBeat(priceUpdateTimeout)
	s.askPriceHeartBeat = types.NewPriceHeartBeat(priceUpdateTimeout)

	s.debtQuotaCache = fixedpoint.NewExpirable(fixedpoint.Zero, time.Time{})

	s.logger = logrus.WithFields(
		logrus.Fields{
			"symbol":      s.Symbol,
			"strategy":    ID,
			"strategy_id": s.InstanceID(),
		},
	)

	s.metricsLabels = prometheus.Labels{
		"strategy_type": ID,
		"strategy_id":   s.InstanceID(),
		"exchange":      s.MakerExchange,
		"symbol":        s.Symbol,
	}

	s.cancelOrderDurationMetrics = cancelOrderDurationMetrics.With(s.metricsLabels)
	s.aggregatedSignalMetrics = aggregatedSignalMetrics.With(s.metricsLabels)
	s.askMarginMetrics = askMarginMetrics.With(s.metricsLabels)
	s.bidMarginMetrics = bidMarginMetrics.With(s.metricsLabels)

	s.makerOrderPlacementDurationMetrics = makerOrderPlacementDurationMetrics.With(s.metricsLabels)
	s.openOrderBidExposureInUsdMetrics = openOrderBidExposureInUsdMetrics.With(s.metricsLabels)
	s.openOrderAskExposureInUsdMetrics = openOrderAskExposureInUsdMetrics.With(s.metricsLabels)

	if s.SignalReverseSideMargin != nil && s.SignalReverseSideMargin.Scale != nil {
		scale, err := s.SignalReverseSideMargin.Scale.Scale()
		if err != nil {
			return err
		}

		if solveErr := scale.Solve(); solveErr != nil {
			return solveErr
		}
	}

	if s.SignalTrendSideMarginDiscount != nil && s.SignalTrendSideMarginDiscount.Scale != nil {
		scale, err := s.SignalTrendSideMarginDiscount.Scale.Scale()
		if err != nil {
			return err
		}

		if solveErr := scale.Solve(); solveErr != nil {
			return solveErr
		}
	}

	if s.DelayedHedge != nil && s.DelayedHedge.DynamicDelayScale != nil {
		if scale, _ := s.DelayedHedge.DynamicDelayScale.Scale(); scale != nil {
			if err := scale.Solve(); err != nil {
				return err
			}
		}
	}

	s.positionExposure = newPositionExposure(s.Symbol)
	s.positionExposure.SetMetricsLabels(ID, s.InstanceID(), s.MakerExchange, s.Symbol)
	for _, sig := range s.SignalConfigList.Signals {
		s.logger.Infof("using signal provider: %s", sig)
	}
	return nil
}

func (s *Strategy) PrintConfig(f io.Writer, pretty bool, withColor ...bool) {
	var tableStyle *table.Style
	if pretty {
		tableStyle = style.NewDefaultTableStyle()
	}

	dynamic.PrintConfig(s, f, tableStyle, len(withColor) > 0 && withColor[0], dynamic.DefaultWhiteList()...)
}

// getBollingerTrend returns -1 when the price is in the downtrend, 1 when the price is in the uptrend, 0 when the price is in the band
func (s *Strategy) getBollingerTrend(quote *Quote) int {
	// when bid price is lower than the down band, then it's in the downtrend
	// when ask price is higher than the up band, then it's in the uptrend
	lastDownBand := fixedpoint.NewFromFloat(s.boll.DownBand.Last(0))
	lastUpBand := fixedpoint.NewFromFloat(s.boll.UpBand.Last(0))

	if quote.BestAskPrice.Compare(lastDownBand) < 0 {
		return -1
	} else if quote.BestBidPrice.Compare(lastUpBand) > 0 {
		return 1
	} else {
		return 0
	}
}

// setPositionStartTime sets the position start time only if it's not set
func (s *Strategy) setPositionStartTime(now time.Time) {
	s.positionStartedAtMutex.Lock()
	if s.positionStartedAt == nil {
		s.positionStartedAt = &now
	}

	s.positionStartedAtMutex.Unlock()
}

func (s *Strategy) resetPositionStartTime() {
	s.positionStartedAtMutex.Lock()
	s.positionStartedAt = nil
	s.positionStartedAtMutex.Unlock()
}

func (s *Strategy) getPositionHoldingPeriod(now time.Time) (time.Duration, bool) {
	s.positionStartedAtMutex.Lock()
	startedAt := s.positionStartedAt
	s.positionStartedAtMutex.Unlock()

	if startedAt == nil || startedAt.IsZero() {
		return 0, false
	}

	return now.Sub(*startedAt), true
}

func (s *Strategy) applySignalMargin(ctx context.Context, quote *Quote) error {
	sig, err := s.aggregateSignal(ctx)
	if err != nil {
		return err
	}

	s.lastAggregatedSignal.Set(sig)
	s.logger.Infof("aggregated signal: %f", sig)

	if sig == 0.0 {
		return nil
	}

	signalAbs := math.Abs(sig)

	var trendSideMarginDiscount, reverseSideMargin float64
	var trendSideMarginDiscountFp, reverseSideMarginFp fixedpoint.Value
	if s.SignalTrendSideMarginDiscount != nil && s.SignalTrendSideMarginDiscount.Enabled {
		trendSideMarginScale, err := s.SignalTrendSideMarginDiscount.Scale.Scale()
		if err != nil {
			return err
		}

		if signalAbs > s.SignalTrendSideMarginDiscount.Threshold {
			// trendSideMarginDiscount is the discount for the trend side margin
			trendSideMarginDiscount = trendSideMarginScale.Call(math.Abs(sig))
			trendSideMarginDiscountFp = fixedpoint.NewFromFloat(trendSideMarginDiscount)

			if sig > 0.0 {
				quote.BidMargin = quote.BidMargin.Sub(trendSideMarginDiscountFp)
			} else if sig < 0.0 {
				quote.AskMargin = quote.AskMargin.Sub(trendSideMarginDiscountFp)
			}
		}
	}

	if s.SignalReverseSideMargin != nil && s.SignalReverseSideMargin.Enabled {
		reverseSideMarginScale, err := s.SignalReverseSideMargin.Scale.Scale()
		if err != nil {
			return err
		}

		if signalAbs > s.SignalReverseSideMargin.Threshold {
			reverseSideMargin = reverseSideMarginScale.Call(math.Abs(sig))
			reverseSideMarginFp = fixedpoint.NewFromFloat(reverseSideMargin)
			if sig < 0.0 {
				quote.BidMargin = quote.BidMargin.Add(reverseSideMarginFp)
			} else if sig > 0.0 {
				quote.AskMargin = quote.AskMargin.Add(reverseSideMarginFp)
			}
		}
	}

	s.logger.Infof(
		"signal margin params: signal = %f, reverseSideMargin = %f, trendSideMarginDiscount = %f", sig,
		reverseSideMargin, trendSideMarginDiscount,
	)

	s.logger.Infof(
		"calculated signal margin: signal = %f, askMargin = %s, bidMargin = %s",
		sig,
		quote.AskMargin,
		quote.BidMargin,
	)

	if s.MinMargin != nil {
		quote.AskMargin = fixedpoint.Max(*s.MinMargin, quote.AskMargin)
		quote.BidMargin = fixedpoint.Max(*s.MinMargin, quote.BidMargin)
	}

	s.logger.Infof(
		"final signal margin: signal = %f, askMargin = %s, bidMargin = %s",
		sig,
		quote.AskMargin,
		quote.BidMargin,
	)

	return nil
}

// applyBollingerMargin applies the bollinger band margin to the quote
func (s *Strategy) applyBollingerMargin(
	quote *Quote,
) error {
	lastDownBand := fixedpoint.NewFromFloat(s.boll.DownBand.Last(0))
	lastUpBand := fixedpoint.NewFromFloat(s.boll.UpBand.Last(0))

	if lastUpBand.IsZero() || lastDownBand.IsZero() {
		s.logger.Warnf("bollinger band value is zero, skipping")
		return nil
	}

	factor := fixedpoint.Min(s.BollBandMarginFactor, fixedpoint.One)
	switch s.getBollingerTrend(quote) {
	case -1:
		// for the downtrend, increase the bid margin
		//  ratio here should be greater than 1.00
		ratio := fixedpoint.Min(lastDownBand.Div(quote.BestAskPrice), fixedpoint.One)

		// so that 1.x can multiply the original bid margin
		bollMargin := s.BollBandMargin.Mul(ratio).Mul(factor)

		s.logger.Infof(
			"%s bollband downtrend: increasing bid margin %f (bidMargin) + %f (bollMargin) = %f (finalBidMargin)",
			s.Symbol,
			quote.BidMargin.Float64(),
			bollMargin.Float64(),
			quote.BidMargin.Add(bollMargin).Float64(),
		)

		quote.BidMargin = quote.BidMargin.Add(bollMargin)
		quote.BidLayerPips = quote.BidLayerPips.Mul(ratio)

	case 1:
		// for the uptrend, increase the ask margin
		// ratio here should be greater than 1.00
		ratio := fixedpoint.Min(quote.BestAskPrice.Div(lastUpBand), fixedpoint.One)

		// so that the original bid margin can be multiplied by 1.x
		bollMargin := s.BollBandMargin.Mul(ratio).Mul(factor)

		s.logger.Infof(
			"%s bollband uptrend adjusting ask margin %f (askMargin) + %f (bollMargin) = %f (finalAskMargin)",
			s.Symbol,
			quote.AskMargin.Float64(),
			bollMargin.Float64(),
			quote.AskMargin.Add(bollMargin).Float64(),
		)

		quote.AskMargin = quote.AskMargin.Add(bollMargin)
		quote.AskLayerPips = quote.AskLayerPips.Mul(ratio)

	default:
		// default, in the band

	}

	return nil
}

// TODO: move this aggregateSignal to the signal package
func (s *Strategy) aggregateSignal(ctx context.Context) (float64, error) {
	sum := 0.0
	voters := 0.0
	for _, signalWrapper := range s.SignalConfigList.Signals {
		sig, err := signalWrapper.Signal.CalculateSignal(ctx)
		if err != nil {
			return 0, err
		} else if sig == 0.0 {
			continue
		}

		if signalWrapper.Weight > 0.0 {
			sum += sig * signalWrapper.Weight
			voters += signalWrapper.Weight
		} else {
			sum += sig
			voters++
		}
	}

	if sum == 0.0 || voters == 0.0 {
		return 0.0, nil
	}

	return sum / voters, nil
}

// getInitialLayerQuantity returns the initial quantity for the layer
// i is the layer index, starting from 0
func (s *Strategy) getInitialLayerQuantity(i int) (fixedpoint.Value, error) {
	if s.QuantityScale != nil {
		qf, err := s.QuantityScale.Scale(i + 1)
		if err != nil {
			return fixedpoint.Zero, fmt.Errorf("quantityScale error: %w", err)
		}

		s.logger.Infof("%s scaling bid #%d quantity to %f", s.Symbol, i+1, qf)

		// override the default quantity
		return fixedpoint.NewFromFloat(qf), nil
	}

	q := s.Quantity

	if s.QuantityMultiplier.Sign() > 0 && i > 0 {
		q = fixedpoint.NewFromFloat(
			q.Float64() * math.Pow(
				s.QuantityMultiplier.Float64(), float64(i+1),
			),
		)
	}

	// fallback to the fixed quantity
	return q, nil
}

const debtQuotaCacheDuration = 30 * time.Second

// margin level = totalValue / totalDebtValue * MMR (maintenance margin ratio)
// on binance:
// - MMR with 10x leverage = 5%
// - MMR with 5x leverage = 9%
// - MMR with 3x leverage = 10%
func (s *Strategy) calculateDebtQuota(totalValue, debtValue, minMarginLevel, leverage fixedpoint.Value) fixedpoint.Value {
	now := time.Now()
	if s.debtQuotaCache != nil {
		if v, ok := s.debtQuotaCache.Get(now); ok {
			return v
		}
	}

	if minMarginLevel.IsZero() || totalValue.IsZero() {
		return fixedpoint.Zero
	}

	defaultMmr := fixedpoint.NewFromFloat(9.0 * 0.01)
	if leverage.Compare(fixedpoint.NewFromFloat(10.0)) >= 0 {
		defaultMmr = fixedpoint.NewFromFloat(5.0 * 0.01) // 5%
	} else if leverage.Compare(fixedpoint.NewFromFloat(5.0)) >= 0 {
		defaultMmr = fixedpoint.NewFromFloat(9.0 * 0.01) // 9%
	} else if leverage.Compare(fixedpoint.NewFromFloat(3.0)) >= 0 {
		defaultMmr = fixedpoint.NewFromFloat(10.0 * 0.01) // 10%
	}

	debtCap := totalValue.Div(minMarginLevel).Div(defaultMmr)
	marginLevel := totalValue.Div(debtValue).Div(defaultMmr)

	s.logger.Infof(
		"calculateDebtQuota: debtCap=%f, debtValue=%f currentMarginLevel=%f mmr=%f",
		debtCap.Float64(),
		debtValue.Float64(),
		marginLevel.Float64(),
		defaultMmr.Float64(),
	)

	debtQuota := debtCap.Sub(debtValue)
	if debtQuota.Sign() < 0 {
		return fixedpoint.Zero
	}

	if s.debtQuotaCache == nil {
		s.debtQuotaCache = fixedpoint.NewExpirable(debtQuota, now.Add(debtQuotaCacheDuration))
	} else {
		s.debtQuotaCache.Set(debtQuota, now.Add(debtQuotaCacheDuration))
	}

	return debtQuota
}

func (s *Strategy) allowMarginHedge(
	session *bbgo.ExchangeSession,
	minMarginLevel, maxHedgeAccountLeverage fixedpoint.Value,
	makerSide types.SideType,
) (bool, fixedpoint.Value) {
	zero := fixedpoint.Zero

	hedgeAccount := session.GetAccount()
	if hedgeAccount.MarginLevel.IsZero() || minMarginLevel.IsZero() {
		return false, zero
	}

	lastPrice := s.lastPrice.Get()
	bufMinMarginLevel := minMarginLevel.Mul(fixedpoint.NewFromFloat(1.005))

	accountValueCalculator := session.GetAccountValueCalculator()
	marketValue := accountValueCalculator.MarketValue()
	debtValue := accountValueCalculator.DebtValue()
	netValueInUsd := accountValueCalculator.NetValue()

	s.logger.Infof(
		"hedge account net value in usd: %f, debt value in usd: %f, total value in usd: %f",
		netValueInUsd.Float64(),
		debtValue.Float64(),
		marketValue.Float64(),
	)

	// if the margin level is higher than the minimal margin level,
	// we can hedge the position, but we need to check the debt quota
	if hedgeAccount.MarginLevel.Compare(minMarginLevel) > 0 {

		// debtQuota is the quota with minimal margin level
		debtQuota := s.calculateDebtQuota(marketValue, debtValue, bufMinMarginLevel, maxHedgeAccountLeverage)

		s.logger.Infof(
			"hedge account margin level %f > %f, debt quota: %f",
			hedgeAccount.MarginLevel.Float64(), minMarginLevel.Float64(), debtQuota.Float64(),
		)

		if debtQuota.Sign() <= 0 {
			return false, zero
		}

		// if MaxHedgeAccountLeverage is set, we need to calculate credit buffer
		if maxHedgeAccountLeverage.Sign() > 0 {
			maximumValueInUsd := netValueInUsd.Mul(maxHedgeAccountLeverage)
			leverageQuotaInUsd := maximumValueInUsd.Sub(debtValue)
			s.logger.Infof(
				"hedge account maximum leveraged value in usd: %f (%f x), quota in usd: %f",
				maximumValueInUsd.Float64(),
				maxHedgeAccountLeverage.Float64(),
				leverageQuotaInUsd.Float64(),
			)

			debtQuota = fixedpoint.Min(debtQuota, leverageQuotaInUsd)
		}

		switch makerSide {
		case types.SideTypeBuy:
			return true, debtQuota

		case types.SideTypeSell:
			if lastPrice.IsZero() {
				return false, zero
			}

			return true, debtQuota.Div(lastPrice)

		}
		return true, zero
	}

	// makerSide here is the makerSide of maker
	// if the margin level is too low, check if we can hedge the position with repayments to reduce the position
	quoteBal, _ := hedgeAccount.Balance(s.sourceMarket.QuoteCurrency)
	baseBal, _ := hedgeAccount.Balance(s.sourceMarket.BaseCurrency)

	switch makerSide {
	case types.SideTypeBuy:
		if baseBal.Available.IsZero() {
			return false, zero
		}

		quota := baseBal.Available.Mul(lastPrice)

		// for buy orders, we need to check if we can repay the quoteBal asset via selling the base balance
		quoteDebt := quoteBal.Debt()
		if quoteDebt.Sign() > 0 {
			return true, fixedpoint.Min(quota, quoteDebt)
		}

		return false, zero

	case types.SideTypeSell:
		if quoteBal.Available.IsZero() {
			return false, zero
		}

		quota := quoteBal.Available.Div(lastPrice)

		baseDebt := baseBal.Debt()
		if baseDebt.Sign() > 0 {
			// return how much quote bal amount we can use to place the buy order
			return true, fixedpoint.Min(quota, baseDebt)
		}

		return false, zero
	}

	return false, zero
}

func (s *Strategy) updateQuote(ctx context.Context) error {
	cancelMakerOrdersProfile := timeprofile.Start("cancelMakerOrders")

	if err := s.activeMakerOrders.GracefulCancel(ctx, s.makerSession.Exchange); err != nil {
		s.logger.Warnf("there are some %s orders not canceled, skipping placing maker orders", s.Symbol)
		s.activeMakerOrders.Print()
		return nil
	}

	s.cancelOrderDurationMetrics.Observe(float64(cancelMakerOrdersProfile.Stop().Milliseconds()))

	if s.activeMakerOrders.NumOfOrders() > 0 {
		s.logger.Warnf("unable to cancel all %s orders, skipping placing maker orders", s.Symbol)
		return nil
	}

	if !s.sourceSession.Connectivity.IsConnected() {
		s.logger.Warnf("source session is disconnected, skipping update quote")
		return nil
	}

	if !s.makerSession.Connectivity.IsConnected() {
		s.logger.Warnf("maker session is disconnected, skipping update quote")
		return nil
	}

	sig, err := s.aggregateSignal(ctx)
	if err != nil {
		return err
	}

	s.logger.Infof("aggregated signal: %f", sig)
	s.aggregatedSignalMetrics.Set(sig)

	now := time.Now()
	if s.CircuitBreaker != nil {
		if reason, halted := s.CircuitBreaker.IsHalted(now); halted {
			instance := s.InstanceID()

			s.logger.Warnf("strategy %s is halted, reason: %s", instance, reason)

			if s.circuitBreakerAlertLimiter.AllowN(now, 1) {
				bbgo.Notify("Strategy %s is halted, reason: %s", instance, reason)
			}

			// make sure spread maker order is canceled
			if s.SpreadMaker != nil && s.SpreadMaker.Enabled {
				s.cancelSpreadMakerOrderAndReturnCoveredPos(ctx)
			}

			return nil
		}
	}

	bestBidPrice := fixedpoint.Zero
	bestAskPrice := fixedpoint.Zero

	if s.SyntheticHedge != nil && s.SyntheticHedge.Enabled {
		bestBid, bestAsk, hasPrice := s.SyntheticHedge.GetQuotePrices()
		if !hasPrice {
			s.logger.Warnf("no valid price, skip quoting")
			return nil
		}

		bestBidPrice = bestBid
		bestAskPrice = bestAsk
	} else {
		bestBid, bestAsk, hasPrice := s.sourceBook.BestBidAndAsk()
		if !hasPrice {
			s.logger.Warnf("no valid price, skip quoting")
			return nil
		}

		bookLastUpdateTime := s.sourceBook.LastUpdateTime()
		if _, err := s.bidPriceHeartBeat.Update(bestBid); err != nil {
			s.logger.WithError(err).Errorf(
				"quote update error, %s price not updating, order book last update: %s ago",
				s.Symbol,
				time.Since(bookLastUpdateTime),
			)

			s.sourceSession.MarketDataStream.Reconnect()
			s.sourceBook.Reset()
			return err
		}

		if _, err := s.askPriceHeartBeat.Update(bestAsk); err != nil {
			s.logger.WithError(err).Errorf(
				"quote update error, %s price not updating, order book last update: %s ago",
				s.Symbol,
				time.Since(bookLastUpdateTime),
			)

			s.sourceSession.MarketDataStream.Reconnect()
			s.sourceBook.Reset()
			return err
		}

		bestBidPrice = bestBid.Price
		bestAskPrice = bestAsk.Price
	}

	s.logger.Infof("%s book ticker: best ask / best bid = %v / %v", s.Symbol, bestAskPrice, bestBidPrice)

	if bestBidPrice.Compare(bestAskPrice) > 0 {
		return fmt.Errorf(
			"best bid price %f is higher than best ask price %f, skip quoting",
			bestBidPrice.Float64(),
			bestAskPrice.Float64(),
		)
	}

	// use mid-price for the last price
	midPrice := bestBidPrice.Add(bestAskPrice).Div(two)
	s.lastPrice.Set(midPrice)
	s.priceSolver.Update(s.Symbol, midPrice)

	sourceBook := s.sourceBook.CopyDepth(10)
	if valid, err := sourceBook.IsValid(); !valid {
		s.logger.WithError(err).Errorf("%s invalid copied order book, skip quoting: %v", s.Symbol, err)
		return err
	}

	var disableMakerBid = false
	var disableMakerAsk = false

	// check maker's balance quota
	// we load the balances from the account while we're generating the orders,
	// the balance may have a chance to be deducted by other strategies or manual orders submitted by the user
	makerBalances := s.makerSession.GetAccount().Balances().NotZero()

	s.logger.Infof("maker balances: %+v", makerBalances)

	makerQuota := &bbgo.QuotaTransaction{}
	if b, ok := makerBalances[s.makerMarket.BaseCurrency]; ok {
		if s.makerMarket.IsDustQuantity(b.Available, s.lastPrice.Get()) {
			disableMakerAsk = true
			s.logger.Infof("%s maker ask disabled: insufficient base balance %s", s.Symbol, b.String())
		} else {
			makerQuota.BaseAsset.Add(b.Available)
		}
	} else {
		disableMakerAsk = true
		s.logger.Infof("%s maker ask disabled: base balance %s not found", s.Symbol, b.String())
	}

	if b, ok := makerBalances[s.makerMarket.QuoteCurrency]; ok {
		if b.Available.Compare(s.makerMarket.MinNotional) > 0 {
			if s.MaxQuoteQuotaRatio.Sign() > 0 {
				quoteAvailable := b.Total().Mul(s.MaxQuoteQuotaRatio)
				makerQuota.QuoteAsset.Add(quoteAvailable)
			} else {
				// use all quote balances as much as possible
				makerQuota.QuoteAsset.Add(b.Available)
			}

		} else {
			disableMakerBid = true
			s.logger.Infof("%s maker bid disabled: insufficient quote balance %s", s.Symbol, b.String())
		}
	} else {
		disableMakerBid = true
		s.logger.Infof("%s maker bid disabled: quote balance %s not found", s.Symbol, b.String())
	}

	s.logger.Infof("maker quota: %+v", makerQuota)

	// if
	//  1) the source session is a margin session
	//  2) the min margin level is configured
	//  3) the hedge account's margin level is lower than the min margin level
	hedgeAccount := s.sourceSession.GetAccount()
	hedgeBalances := hedgeAccount.Balances()
	hedgeQuota := &bbgo.QuotaTransaction{}

	s.logger.Infof("hedge balances: %+v", hedgeBalances.NotZero())

	if s.sourceSession.Margin &&
		!s.MinMarginLevel.IsZero() &&
		!hedgeAccount.MarginLevel.IsZero() {

		if hedgeAccount.MarginLevel.Compare(s.MinMarginLevel) < 0 {
			s.logger.Warnf(
				"hedge account margin level %s is less then the min margin level %s, trying to repay the debts...",
				hedgeAccount.MarginLevel.String(),
				s.MinMarginLevel.String(),
			)

			tryToRepayDebts(ctx, s.sourceSession)
		} else {
			s.logger.Infof(
				"hedge account margin level %s is greater than the min margin level %s, calculating the net value",
				hedgeAccount.MarginLevel.String(),
				s.MinMarginLevel.String(),
			)
		}

		allowMarginSell, bidQuota := s.allowMarginHedge(s.sourceSession, s.MinMarginLevel, s.MaxHedgeAccountLeverage, types.SideTypeBuy)
		if allowMarginSell {
			hedgeQuota.BaseAsset.Add(bidQuota.Div(bestBidPrice))
		} else {
			s.logger.Warnf("margin hedge sell is disabled, disabling maker bid orders...")
			disableMakerBid = true
		}

		allowMarginBuy, sellQuota := s.allowMarginHedge(s.sourceSession, s.MinMarginLevel, s.MaxHedgeAccountLeverage, types.SideTypeSell)
		if allowMarginBuy {
			hedgeQuota.QuoteAsset.Add(sellQuota.Mul(bestAskPrice))
		} else {
			s.logger.Warnf("margin hedge buy is disabled, disabling maker ask orders...")
			disableMakerAsk = true
		}

		if disableMakerBid || disableMakerAsk {
			tryToRepayDebts(ctx, s.sourceSession)
		}
	} else {
		if b, ok := hedgeBalances[s.sourceMarket.BaseCurrency]; ok {
			// to make bid orders, we need enough base asset in the foreign exchange,
			// if the base asset balance is not enough for selling
			if s.StopHedgeBaseBalance.Sign() > 0 {
				minAvailable := s.StopHedgeBaseBalance.Add(s.sourceMarket.MinQuantity)
				if b.Available.Compare(minAvailable) > 0 {
					hedgeQuota.BaseAsset.Add(b.Available.Sub(minAvailable))
				} else {
					s.logger.Warnf("%s maker bid disabled: insufficient hedge base balance %s", s.Symbol, b.String())
					disableMakerBid = true
				}
			} else if b.Available.Compare(s.sourceMarket.MinQuantity) > 0 {
				hedgeQuota.BaseAsset.Add(b.Available)
			} else {
				s.logger.Warnf("%s maker bid disabled: insufficient hedge base balance %s", s.Symbol, b.String())
				disableMakerBid = true
			}
		}

		if b, ok := hedgeBalances[s.sourceMarket.QuoteCurrency]; ok {
			// to make ask orders, we need enough quote asset in the foreign exchange,
			// if the quote asset balance is not enough for buying
			if s.StopHedgeQuoteBalance.Sign() > 0 {
				minAvailable := s.StopHedgeQuoteBalance.Add(s.sourceMarket.MinNotional)
				if b.Available.Compare(minAvailable) > 0 {
					hedgeQuota.QuoteAsset.Add(b.Available.Sub(minAvailable))
				} else {
					s.logger.Warnf("%s maker ask disabled: insufficient hedge quote balance %s", s.Symbol, b.String())
					disableMakerAsk = true
				}
			} else if b.Available.Compare(s.sourceMarket.MinNotional) > 0 {
				hedgeQuota.QuoteAsset.Add(b.Available)
			} else {
				s.logger.Warnf("%s maker ask disabled: insufficient hedge quote balance %s", s.Symbol, b.String())
				disableMakerAsk = true
			}
		}
	}

	// if max exposure position is configured, we should not:
	// 1. place bid orders when we already bought too much
	// 2. place ask orders when we already sold too much
	if s.MaxExposurePosition.Sign() > 0 {
		pos := s.Position.GetBase()

		if pos.Compare(s.MaxExposurePosition.Neg()) <= 0 {
			// stop sell if we over-sell
			disableMakerAsk = true
			s.logger.Warnf(
				"%s ask maker is disabled: %f exceeded max exposure %f", s.Symbol, pos.Float64(),
				s.MaxExposurePosition.Float64(),
			)
		} else if pos.Compare(s.MaxExposurePosition) >= 0 {
			// stop buy if we over buy
			disableMakerBid = true
			s.logger.Warnf(
				"%s bid maker is disabled: %f exceeded max exposure %f", s.Symbol, pos.Float64(),
				s.MaxExposurePosition.Float64(),
			)
		}
	}

	if disableMakerAsk && disableMakerBid {
		s.logger.Warnf("%s bid/ask maker is disabled", s.Symbol)
		return nil
	}

	var orderType = types.OrderTypeLimit
	if s.MakerOnly {
		orderType = types.OrderTypeLimitMaker
	}

	var submitOrders []types.SubmitOrder

	var quote = &Quote{
		BestBidPrice: bestBidPrice,
		BestAskPrice: bestAskPrice,
		BidMargin:    s.BidMargin,
		AskMargin:    s.AskMargin,
		BidLayerPips: s.Pips,
		AskLayerPips: s.Pips,
	}

	if s.EnableSignalMargin {
		if err := s.applySignalMargin(ctx, quote); err != nil {
			s.logger.WithError(err).Errorf("unable to apply signal margin")
		}

	} else if s.EnableBollBandMargin {
		if err := s.applyBollingerMargin(quote); err != nil {
			s.logger.WithError(err).Errorf("unable to apply bollinger margin")
		}
	}

	bidExposureInUsd := fixedpoint.Zero
	askExposureInUsd := fixedpoint.Zero

	s.bidMarginMetrics.Set(quote.BidMargin.Float64())
	s.askMarginMetrics.Set(quote.AskMargin.Float64())

	if s.EnableArbitrage {
		done, err := s.tryArbitrage(ctx, quote, makerBalances, hedgeBalances, disableMakerBid, disableMakerAsk)
		if err != nil {
			s.logger.WithError(err).Errorf("unable to arbitrage")
		} else if done {
			return nil
		}
	}

	var bidSourcePricer = pricer.FromBestPrice(types.SideTypeBuy, s.sourceBook)
	coverBidDepth := nopCover
	if s.UseDepthPrice {
		bidCoveredDepth := pricer.NewCoveredDepth(s.depthSourceBook, types.SideTypeBuy, s.DepthQuantity)
		bidSourcePricer = bidCoveredDepth.Pricer()
		coverBidDepth = bidCoveredDepth.Cover
	}

	bidPricer := pricer.Compose(append([]pricer.Pricer{bidSourcePricer}, pricer.Compose(
		pricer.ApplyMargin(types.SideTypeBuy, quote.BidMargin),
		pricer.ApplyFeeRate(types.SideTypeBuy, s.makerSession.MakerFeeRate),
		pricer.AdjustByTick(types.SideTypeBuy, quote.BidLayerPips, s.makerMarket.TickSize),
	))...)

	var askSourcePricer = pricer.FromBestPrice(types.SideTypeSell, s.sourceBook)
	coverAskDepth := nopCover
	if s.UseDepthPrice {
		askCoveredDepth := pricer.NewCoveredDepth(s.depthSourceBook, types.SideTypeSell, s.DepthQuantity)
		coverAskDepth = askCoveredDepth.Cover
		askSourcePricer = askCoveredDepth.Pricer()
	}

	askPricer := pricer.Compose(append([]pricer.Pricer{askSourcePricer}, pricer.Compose(
		pricer.ApplyMargin(types.SideTypeSell, quote.AskMargin),
		pricer.ApplyFeeRate(types.SideTypeSell, s.makerSession.MakerFeeRate),
		pricer.AdjustByTick(types.SideTypeSell, quote.BidLayerPips, s.makerMarket.TickSize),
	))...)

	if !disableMakerBid {
		for i := 0; i < s.NumLayers; i++ {
			bidQuantity, err := s.getInitialLayerQuantity(i)
			if err != nil {
				return err
			}

			// for maker bid orders
			coverBidDepth(bidQuantity)

			bidPrice := fixedpoint.Zero
			if s.SyntheticHedge != nil && s.SyntheticHedge.Enabled {
				// note: the accumulativeBidQuantity is not used yet,
				// reason: the fiat market base unit is different from the maker market.
				if bid, _, ok := s.SyntheticHedge.GetQuotePrices(); ok {
					bidPrice = bid
				} else {
					s.logger.Warnf("no valid synthetic price")
				}
			} else {
				bidPrice = bidPricer(i, bestBidPrice)
			}

			if bidPrice.IsZero() {
				s.logger.Warnf("no valid bid price, skip quoting")
				break
			}

			if i == 0 {
				s.logger.Infof("maker best bid price %f", bidPrice.Float64())
				makerBestBidPriceMetrics.With(s.metricsLabels).Set(bidPrice.Float64())
			}

			requiredQuote := fixedpoint.Min(
				bidQuantity.Mul(bidPrice),
				makerQuota.QuoteAsset.Available,
			)

			requiredQuote = s.makerMarket.TruncateQuoteQuantity(requiredQuote)
			bidQuantity = s.makerMarket.TruncateQuantity(requiredQuote.Div(bidPrice))

			if s.makerMarket.IsDustQuantity(bidQuantity, bidPrice) {
				continue
			}

			// if we bought, then we need to sell the base from the hedge session
			// if the hedge session is a margin session, we don't need to lock the base asset
			if makerQuota.QuoteAsset.Lock(requiredQuote) &&
				(s.sourceSession.Margin || hedgeQuota.BaseAsset.Lock(bidQuantity)) {

				// if we bought, then we need to sell the base from the hedge session
				submitOrders = append(
					submitOrders, types.SubmitOrder{
						Symbol:      s.Symbol,
						Market:      s.makerMarket,
						Type:        orderType,
						Side:        types.SideTypeBuy,
						Price:       bidPrice,
						Quantity:    bidQuantity,
						TimeInForce: types.TimeInForceGTC,
						GroupID:     s.groupID,
					},
				)

				makerQuota.Commit()
				hedgeQuota.Commit()
				bidExposureInUsd = bidExposureInUsd.Add(requiredQuote)
			} else {
				makerQuota.Rollback()
				hedgeQuota.Rollback()
			}

		}
	}

	// for maker ask orders
	if !disableMakerAsk {
		for i := 0; i < s.NumLayers; i++ {
			askQuantity, err := s.getInitialLayerQuantity(i)
			if err != nil {
				return err
			}

			coverAskDepth(askQuantity)

			askPrice := fixedpoint.Zero
			if s.SyntheticHedge != nil && s.SyntheticHedge.Enabled {
				if _, ask, ok := s.SyntheticHedge.GetQuotePrices(); ok {
					askPrice = ask
				} else {
					s.logger.Warnf("no valid synthetic price")
				}
			} else {
				askPrice = askPricer(i, bestAskPrice)
			}

			if askPrice.IsZero() {
				s.logger.Warnf("no valid ask price, skip quoting")
				break
			}

			if i == 0 {
				s.logger.Infof("maker best ask price %f", askPrice.Float64())
				makerBestAskPriceMetrics.With(s.metricsLabels).Set(askPrice.Float64())
			}

			requiredBase := fixedpoint.Min(askQuantity, makerQuota.BaseAsset.Available)
			askQuantity = s.makerMarket.TruncateQuantity(requiredBase)

			if s.makerMarket.IsDustQuantity(askQuantity, askPrice) {
				continue
			}

			if makerQuota.BaseAsset.Lock(requiredBase) &&
				(s.sourceSession.Margin || hedgeQuota.QuoteAsset.Lock(requiredBase.Mul(askPrice))) {

				// if we bought, then we need to sell the base from the hedge session
				submitOrders = append(
					submitOrders, types.SubmitOrder{
						Symbol:      s.Symbol,
						Market:      s.makerMarket,
						Type:        orderType,
						Side:        types.SideTypeSell,
						Price:       askPrice,
						Quantity:    askQuantity,
						TimeInForce: types.TimeInForceGTC,
						GroupID:     s.groupID,
					},
				)
				makerQuota.Commit()
				hedgeQuota.Commit()

				askExposureInUsd = askExposureInUsd.Add(requiredBase.Mul(askPrice))
			} else {
				makerQuota.Rollback()
				hedgeQuota.Rollback()
			}

			if s.QuantityMultiplier.Sign() > 0 {
				askQuantity = askQuantity.Mul(s.QuantityMultiplier)
			}
		}
	}

	if len(submitOrders) == 0 {
		s.logger.Warnf("no orders generated")
		return nil
	}

	formattedOrders, err := s.makerSession.FormatOrders(submitOrders)
	if err != nil {
		return err
	}

	defer s.tradeCollector.Process()

	makerOrderPlacementProfile := timeprofile.Start("makerOrderPlacement")
	createdOrders, errIdx, err := bbgo.BatchPlaceOrder(
		ctx, s.makerSession.Exchange, s.makerOrderCreateCallback, formattedOrders...,
	)
	if err != nil {
		s.logger.WithError(err).Errorf("unable to place maker orders: %+v", formattedOrders)
		return err
	}

	s.makerOrderPlacementDurationMetrics.Observe(float64(makerOrderPlacementProfile.Stop().Milliseconds()))
	s.openOrderBidExposureInUsdMetrics.Set(bidExposureInUsd.Float64())
	s.openOrderAskExposureInUsdMetrics.Set(askExposureInUsd.Float64())

	_ = errIdx
	_ = createdOrders
	return nil
}

func (s *Strategy) makerOrderCreateCallback(createdOrder types.Order) {
	s.orderStore.Add(createdOrder)
	s.activeMakerOrders.Add(createdOrder)
}

// tryArbitrage tries to arbitrage between the source and maker exchange
func (s *Strategy) tryArbitrage(
	ctx context.Context, quote *Quote, makerBalances, hedgeBalances types.BalanceMap, disableBid, disableAsk bool,
) (bool, error) {
	if s.makerBook == nil {
		return false, nil
	}

	marginBidPrice := quote.BestBidPrice.Mul(fixedpoint.One.Sub(quote.BidMargin))
	marginAskPrice := quote.BestAskPrice.Mul(fixedpoint.One.Add(quote.AskMargin))

	makerBid, makerAsk, ok := s.makerBook.BestBidAndAsk()
	if !ok {
		return false, nil
	}

	var iocOrders []types.SubmitOrder
	if makerAsk.Price.Compare(marginBidPrice) <= 0 {
		quoteBalance, hasQuote := makerBalances[s.makerMarket.QuoteCurrency]
		if !hasQuote || disableBid {
			return false, nil
		}

		availableQuote := s.makerMarket.TruncateQuoteQuantity(quoteBalance.Available)

		askPvs := s.makerBook.SideBook(types.SideTypeSell)
		sumPv := aggregatePriceVolumeSliceWithPriceFilter(types.SideTypeSell, askPvs, marginBidPrice)
		qty := fixedpoint.Min(availableQuote.Div(sumPv.Price), sumPv.Volume)

		if sourceBase, ok := hedgeBalances[s.sourceMarket.BaseCurrency]; ok {
			qty = fixedpoint.Min(qty, sourceBase.Available)
		} else {
			// insufficient hedge base balance for arbitrage
			return false, nil
		}

		if s.makerMarket.IsDustQuantity(qty, sumPv.Price) {
			return false, nil
		}

		iocOrders = append(
			iocOrders, types.SubmitOrder{
				Symbol:      s.Symbol,
				Market:      s.makerMarket,
				Type:        types.OrderTypeLimit,
				Side:        types.SideTypeBuy,
				Price:       sumPv.Price,
				Quantity:    qty,
				TimeInForce: types.TimeInForceIOC,
			},
		)

	} else if makerBid.Price.Compare(marginAskPrice) >= 0 {
		baseBalance, hasBase := makerBalances[s.makerMarket.BaseCurrency]
		if !hasBase || disableAsk {
			return false, nil
		}

		availableBase := s.makerMarket.TruncateQuantity(baseBalance.Available)

		bidPvs := s.makerBook.SideBook(types.SideTypeBuy)
		sumPv := aggregatePriceVolumeSliceWithPriceFilter(types.SideTypeBuy, bidPvs, marginAskPrice)
		qty := fixedpoint.Min(availableBase, sumPv.Volume)

		if sourceQuote, ok := hedgeBalances[s.sourceMarket.QuoteCurrency]; ok {
			qty = fixedpoint.Min(qty, quote.BestAskPrice.Div(sourceQuote.Available))
		} else {
			// insufficient hedge quote balance for arbitrage
			return false, nil
		}

		if s.makerMarket.IsDustQuantity(qty, sumPv.Price) {
			return false, nil
		}

		// send ioc order for arbitrage
		iocOrders = append(
			iocOrders, types.SubmitOrder{
				Symbol:      s.Symbol,
				Market:      s.makerMarket,
				Type:        types.OrderTypeLimit,
				Side:        types.SideTypeSell,
				Price:       sumPv.Price,
				Quantity:    qty,
				TimeInForce: types.TimeInForceIOC,
			},
		)
	}

	if len(iocOrders) == 0 {
		return false, nil
	}

	// send ioc order for arbitrage
	formattedOrders, err := s.makerSession.FormatOrders(iocOrders)
	if err != nil {
		return false, err
	}

	defer s.tradeCollector.Process()

	createdOrders, _, err := bbgo.BatchPlaceOrder(
		ctx,
		s.makerSession.Exchange,
		s.makerOrderCreateCallback,
		formattedOrders...,
	)

	if err != nil {
		return len(createdOrders) > 0, err
	}

	s.logger.Infof("sent arbitrage IOC order: %+v", createdOrders)
	return true, nil
}

func AdjustHedgeQuantityWithAvailableBalance(
	account *types.Account,
	market types.Market,
	side types.SideType, quantity, lastPrice fixedpoint.Value,
) fixedpoint.Value {
	switch side {

	case types.SideTypeBuy:
		// check quote quantity
		if quote, ok := account.Balance(market.QuoteCurrency); ok {
			if quote.Available.Compare(market.MinNotional) < 0 {
				// adjust price to higher 0.1%, so that we can ensure that the order can be executed
				availableQuote := market.TruncateQuoteQuantity(quote.Available)
				quantity = bbgo.AdjustQuantityByMaxAmount(quantity, lastPrice, availableQuote)

			}
		}

	case types.SideTypeSell:
		// check quote quantity
		if base, ok := account.Balance(market.BaseCurrency); ok {
			if base.Available.Compare(quantity) < 0 {
				quantity = base.Available
			}
		}
	}

	// truncate the quantity to the supported precision
	return market.TruncateQuantity(quantity)
}

// canDelayHedge returns true if the hedge can be delayed
func (s *Strategy) canDelayHedge(hedgeSide types.SideType, pos fixedpoint.Value) bool {
	if s.DelayedHedge == nil || !s.DelayedHedge.Enabled {
		return false
	}

	sig := s.lastAggregatedSignal.Get()
	signalAbs := math.Abs(sig)
	if signalAbs < s.DelayedHedge.SignalThreshold {
		return false
	}

	// if the signal is strong enough, we can delay the hedge and wait for the next tick
	period, ok := s.getPositionHoldingPeriod(time.Now())
	if !ok {
		return false
	}

	var maxDelay = s.DelayedHedge.MaxDelayDuration.Duration()
	var delay = s.DelayedHedge.FixedDelayDuration.Duration()

	if s.DelayedHedge.DynamicDelayScale != nil {
		if scale, _ := s.DelayedHedge.DynamicDelayScale.Scale(); scale != nil {
			delay = time.Duration(scale.Call(signalAbs)) * time.Millisecond
		}
	}

	if delay > maxDelay {
		delay = maxDelay
	}

	if (sig > 0 && hedgeSide == types.SideTypeSell) || (sig < 0 && hedgeSide == types.SideTypeBuy) {
		if period < delay {
			s.logger.Infof(
				"delay hedge enabled, signal %f is strong enough, waiting for the next tick to hedge %s quantity (max delay %s)",
				sig, pos, delay,
			)

			delayedHedgeCounterMetrics.With(s.metricsLabels).Inc()
			delayedHedgeMaxDurationMetrics.With(s.metricsLabels).Observe(float64(delay.Milliseconds()))
			return true
		}
	}

	return false
}

func (s *Strategy) cancelSpreadMakerOrderAndReturnCoveredPos(
	ctx context.Context,
) {
	s.logger.Infof("canceling current spread maker order...")

	finalOrder, err := s.SpreadMaker.cancelAndQueryOrder(ctx)
	if err != nil {
		s.logger.WithError(err).Errorf("spread maker: cancel order error")
		return
	}

	// if the final order is nil means the order is already canceled or not found
	if finalOrder == nil {
		return
	}

	spreadMakerVolumeMetrics.With(s.metricsLabels).Add(finalOrder.ExecutedQuantity.Float64())
	spreadMakerQuoteVolumeMetrics.With(s.metricsLabels).Add(finalOrder.ExecutedQuantity.Mul(finalOrder.Price).Float64())

	remainingQuantity := finalOrder.GetRemainingQuantity()

	s.logger.Infof("returning remaining quantity %f to the covered position", remainingQuantity.Float64())
	switch finalOrder.Side {
	case types.SideTypeSell:
		s.positionExposure.Cover(remainingQuantity.Neg())
	case types.SideTypeBuy:
		s.positionExposure.Cover(remainingQuantity)
	}
}

func (s *Strategy) spreadMakerHedge(
	ctx context.Context, sig float64, uncoveredPosition, pos fixedpoint.Value,
) (fixedpoint.Value, error) {
	now := time.Now()

	if s.SpreadMaker == nil || !s.SpreadMaker.Enabled || s.makerBook == nil {
		return pos, nil
	}

	side := types.SideTypeBuy
	if pos.Sign() < 0 {
		side = types.SideTypeSell
	}

	makerBid, makerAsk, hasMakerPrice := s.makerBook.BestBidAndAsk()
	if !hasMakerPrice {
		return pos, nil
	}

	curOrder, hasOrder := s.SpreadMaker.getOrder()

	if makerOrderForm, ok := s.SpreadMaker.canSpreadMaking(
		sig, s.Position, uncoveredPosition, s.makerMarket, makerBid.Price, makerAsk.Price,
	); ok {
		s.logger.Infof(
			"position: %f@%f, maker book bid: %f/%f, spread maker order form: %+v",
			s.Position.GetBase().Float64(),
			s.Position.GetAverageCost().Float64(),
			makerAsk.Price.Float64(),
			makerBid.Price.Float64(),
			makerOrderForm,
		)

		// if we have the existing order, cancel it and return the covered position
		// keptOrder means we kept the current order and we don't need to place a new order
		keptOrder := false
		if hasOrder {
			keptOrder = s.SpreadMaker.shouldKeepOrder(curOrder, now)
			if !keptOrder {
				s.logger.Infof("canceling current spread maker order...")
				s.cancelSpreadMakerOrderAndReturnCoveredPos(ctx)
			}
		}

		if !hasOrder || !keptOrder {
			spreadMakerCounterMetrics.With(s.metricsLabels).Inc()
			s.logger.Infof("placing new spread maker order: %+v...", makerOrderForm)

			retOrder, err := s.SpreadMaker.placeOrder(ctx, makerOrderForm)
			if err != nil {
				s.logger.WithError(err).Errorf("unable to place spread maker order")
			} else if retOrder != nil {
				s.orderStore.Add(*retOrder)
				s.orderStore.Prune(8 * time.Hour)

				s.logger.Infof(
					"spread maker order placed: #%d %f@%f (%s)", retOrder.OrderID, retOrder.Quantity.Float64(),
					retOrder.Price.Float64(), retOrder.Status,
				)

				// the side here is reversed from the position side
				// long position -> sell side order to close the long position
				// short position -> buy side order to close the short position
				switch side {
				case types.SideTypeSell:
					s.positionExposure.Cover(retOrder.Quantity)
					pos = pos.Add(retOrder.Quantity)
				case types.SideTypeBuy:
					s.positionExposure.Cover(retOrder.Quantity.Neg())
					pos = pos.Add(retOrder.Quantity.Neg())
				}
			}
		}
	} else if hasOrder {
		// cancel existing spread maker order if the signal is not strong enough
		if s.SpreadMaker.ReverseSignalOrderCancel {
			if !isSignalSidePosition(sig, s.Position.Side()) {
				s.logger.Infof("canceling current spread maker order due to reversed signal...")
				s.cancelSpreadMakerOrderAndReturnCoveredPos(ctx)
			}
		} else {
			shouldKeep := s.SpreadMaker.shouldKeepOrder(curOrder, now)
			if !shouldKeep {
				s.logger.Infof("canceling current spread maker order...")
				s.cancelSpreadMakerOrderAndReturnCoveredPos(ctx)
			}
		}
	}

	return pos, nil
}

func (s *Strategy) hedge(ctx context.Context, uncoveredPosition fixedpoint.Value) {
	if uncoveredPosition.IsZero() && s.positionExposure.IsClosed() {
		return
	}

	// hedgeDelta is the reverse of the uncovered position
	//
	// if the uncovered position is a positive number, e.g., +10 BTC,
	// then we need to sell 10 BTC, hence call .Neg() here
	// if the uncovered position is a negative number, e.g., -10 BTC,
	// then we need to buy 10 BTC, hence call .Neg() here
	hedgeDelta := uncoveredToDelta(uncoveredPosition)
	side := positionToSide(hedgeDelta)

	sig := s.lastAggregatedSignal.Get()

	var err error
	if s.SpreadMaker != nil && s.SpreadMaker.Enabled {
		hedgeDelta, err = s.spreadMakerHedge(ctx, sig, uncoveredPosition, hedgeDelta)
		if err != nil {
			s.logger.WithError(err).Errorf("unable to place spread maker order")
		}
	}

	if hedgeDelta.IsZero() {
		return
	}

	if s.canDelayHedge(side, hedgeDelta) {
		return
	}

	if s.SplitHedge != nil && s.SplitHedge.Enabled {
		if err := s.SplitHedge.Hedge(ctx, uncoveredPosition); err != nil {
			s.logger.WithError(err).Errorf("unable to hedge via split hedge")
			return
		}
	} else if s.SyntheticHedge != nil && s.SyntheticHedge.Enabled {
		if err := s.SyntheticHedge.Hedge(ctx, uncoveredPosition); err != nil {
			s.logger.WithError(err).Errorf("unable to place synthetic hedge order")
			return
		}
	} else {
		if _, err := s.directHedge(ctx, uncoveredPosition); err != nil {
			s.logger.WithError(err).Errorf("unable to hedge position %s %s %f", s.Symbol, side.String(), hedgeDelta.Float64())
			return
		}
	}

	s.resetPositionStartTime()
}

func (s *Strategy) directHedge(
	ctx context.Context, uncoveredPosition fixedpoint.Value,
) (*types.Order, error) {
	hedgeDelta := uncoveredToDelta(uncoveredPosition)
	quantity := hedgeDelta.Abs()
	side := deltaToSide(hedgeDelta)

	price := s.lastPrice.Get()

	bestBid, bestAsk, ok := s.sourceBook.BestBidAndAsk()
	if ok {
		switch side {
		case types.SideTypeBuy:
			price = bestAsk.Price
		case types.SideTypeSell:
			price = bestBid.Price
		}
	}

	account := s.sourceSession.GetAccount()
	if s.sourceSession.Margin {
		// check the margin level
		if !s.MinMarginLevel.IsZero() && !account.MarginLevel.IsZero() && account.MarginLevel.Compare(s.MinMarginLevel) < 0 {
			err := fmt.Errorf("margin level is too low, current margin level: %f, required margin level: %f",
				account.MarginLevel.Float64(), s.MinMarginLevel.Float64(),
			)
			return nil, err
		}
	} else {
		quantity = AdjustHedgeQuantityWithAvailableBalance(
			account, s.sourceMarket, side, quantity, price,
		)
	}

	if s.MaxHedgeQuoteQuantityPerOrder.Sign() > 0 && !price.IsZero() {
		quantity = s.sourceMarket.AdjustQuantityByMaxAmount(quantity, price, s.MaxHedgeQuoteQuantityPerOrder)
	} else {
		// truncate quantity for the supported precision
		quantity = s.sourceMarket.TruncateQuantity(quantity)
	}

	if s.sourceMarket.IsDustQuantity(quantity, price) {
		s.logger.Infof("skip dust quantity: %s @ price %f", quantity.String(), price.Float64())
		return nil, nil
	}

	if s.hedgeErrorRateReservation != nil {
		if !s.hedgeErrorRateReservation.OK() {
			return nil, nil
		}

		bbgo.Notify("Hit hedge error rate limit, waiting...")
		time.Sleep(s.hedgeErrorRateReservation.Delay())
		s.hedgeErrorRateReservation = nil
	}

	bbgo.Notify("Submitting %s hedge order %s %v", s.Symbol, side.String(), quantity)

	submitOrders := []types.SubmitOrder{
		{
			Market:           s.sourceMarket,
			Symbol:           s.Symbol,
			Type:             types.OrderTypeMarket,
			Side:             side,
			Quantity:         quantity,
			MarginSideEffect: types.SideEffectTypeMarginBuy,
		},
	}

	formattedOrders, err := s.sourceSession.FormatOrders(submitOrders)
	if err != nil {
		return nil, fmt.Errorf("unable to format hedge orders: %w", err)
	}

	orderCreateCallback := func(createdOrder types.Order) {
		s.orderStore.Add(createdOrder)
	}

	defer s.tradeCollector.Process()

	createdOrders, _, err := bbgo.BatchPlaceOrder(
		ctx, s.sourceSession.Exchange, orderCreateCallback, formattedOrders...,
	)

	if err != nil {
		s.hedgeErrorRateReservation = s.hedgeErrorLimiter.Reserve()
		return nil, fmt.Errorf("unable to place order: %w", err)
	}

	if len(createdOrders) == 0 {
		return nil, fmt.Errorf("no hedge orders created")
	}

	createdOrder := createdOrders[0]

	s.logger.Infof("submitted hedge orders: %+v", createdOrder)

	// if it's selling, then we should add a positive position
	switch side {
	case types.SideTypeSell:
		s.positionExposure.Cover(quantity)
	case types.SideTypeBuy:
		s.positionExposure.Cover(quantity.Neg())
	}

	return &createdOrder, nil
}

func (s *Strategy) tradeRecover(ctx context.Context) {
	s.wg.Add(1)
	defer s.wg.Done()

	tradeScanInterval := s.RecoverTradeScanPeriod.Duration()
	if tradeScanInterval == 0 {
		tradeScanInterval = 30 * time.Minute
	}

	tradeScanOverlapBufferPeriod := 5 * time.Minute

	tradeScanTicker := time.NewTicker(tradeScanInterval)
	defer tradeScanTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-tradeScanTicker.C:
			s.logger.Infof("scanning trades from %s ago...", tradeScanInterval)

			if s.RecoverTrade {
				startTime := time.Now().Add(-tradeScanInterval).Add(-tradeScanOverlapBufferPeriod)

				// if source symbol is set, and no split hedge or synthetic hedge is configured,
				// recover trades only when in simple hedge mode
				if s.simpleHedgeMode {
					if err := s.tradeCollector.Recover(
						ctx, s.sourceSession.Exchange.(types.ExchangeTradeHistoryService), s.SourceSymbol, startTime,
					); err != nil {
						s.logger.WithError(err).Errorf("query trades error")
					}
				}

				// always recover trades for the maker session if symbol is set
				if s.Symbol != "" {
					if err := s.tradeCollector.Recover(
						ctx, s.makerSession.Exchange.(types.ExchangeTradeHistoryService), s.Symbol, startTime,
					); err != nil {
						s.logger.WithError(err).Errorf("query trades error")
					}
				}
			}
		}
	}
}

func (s *Strategy) Defaults() error {
	if s.BollBandInterval == "" {
		s.BollBandInterval = types.Interval1m
	}

	if s.MaxDelayHedgeDuration == 0 {
		s.MaxDelayHedgeDuration = types.Duration(10 * time.Second)
	}

	if s.DelayHedgeSignalThreshold == 0.0 {
		s.DelayHedgeSignalThreshold = 0.5
	}

	if s.SourceDepthLevel == "" {
		s.SourceDepthLevel = types.DepthLevelMedium
	}

	if s.BollBandMarginFactor.IsZero() {
		s.BollBandMarginFactor = fixedpoint.One
	}

	if s.BollBandMargin.IsZero() {
		s.BollBandMargin = fixedpoint.NewFromFloat(0.001)
	}

	// configure default values
	if s.UpdateInterval == 0 {
		s.UpdateInterval = types.Duration(time.Second)
	}

	if s.HedgeInterval == 0 {
		s.HedgeInterval = types.Duration(10 * time.Second)
	}

	if s.NumLayers == 0 {
		s.NumLayers = 1
	}

	if s.MinMarginLevel.IsZero() {
		s.MinMarginLevel = fixedpoint.NewFromFloat(3.0)
	}

	if s.MaxHedgeAccountLeverage.IsZero() {
		s.MaxHedgeAccountLeverage = fixedpoint.NewFromFloat(1.2)
	}

	if s.BidMargin.IsZero() {
		if !s.Margin.IsZero() {
			s.BidMargin = s.Margin
		} else {
			s.BidMargin = defaultMargin
		}
	}

	if s.AskMargin.IsZero() {
		if !s.Margin.IsZero() {
			s.AskMargin = s.Margin
		} else {
			s.AskMargin = defaultMargin
		}
	}

	if s.CircuitBreaker == nil {
		s.CircuitBreaker = circuitbreaker.NewBasicCircuitBreaker(ID, s.InstanceID(), s.Symbol)
	} else {
		s.CircuitBreaker.SetMetricsInfo(ID, s.InstanceID(), s.Symbol)
	}

	if s.EnableSignalMargin {
		if s.SignalReverseSideMargin.Scale == nil {
			s.SignalReverseSideMargin.Scale = &bbgo.SlideRule{
				ExpScale: &bbgo.ExponentialScale{
					Domain: [2]float64{0, 2.0},
					Range:  [2]float64{0.00010, 0.00500},
				},
				QuadraticScale: nil,
			}
		}

		if s.SignalTrendSideMarginDiscount.Scale == nil {
			s.SignalTrendSideMarginDiscount.Scale = &bbgo.SlideRule{
				ExpScale: &bbgo.ExponentialScale{
					Domain: [2]float64{0, 2.0},
					Range:  [2]float64{0.00010, 0.00500},
				},
			}
		}

		if s.SignalTrendSideMarginDiscount.Threshold == 0.0 {
			s.SignalTrendSideMarginDiscount.Threshold = 1.0
		}
	}

	if s.DelayedHedge != nil {
		// default value protection for delayed hedge
		if s.DelayedHedge.MaxDelayDuration == 0 {
			s.DelayedHedge.MaxDelayDuration = types.Duration(3 * time.Second)
		}

		if s.DelayedHedge.SignalThreshold == 0.0 {
			s.DelayedHedge.SignalThreshold = 0.5
		}
	}

	if s.MinMarginLevel.IsZero() {
		s.MinMarginLevel = fixedpoint.NewFromFloat(2.0)
	}

	// circuitBreakerAlertLimiter is for CircuitBreaker alerts
	s.circuitBreakerAlertLimiter = rate.NewLimiter(rate.Every(3*time.Minute), 2)
	s.reportProfitStatsRateLimiter = rate.NewLimiter(rate.Every(3*time.Minute), 1)
	s.hedgeErrorLimiter = rate.NewLimiter(rate.Every(1*time.Minute), 1)

	// Set SourceSymbol to Symbol if not set
	if s.SourceSymbol == "" {
		s.SourceSymbol = s.Symbol
	}

	if s.MakerSymbol == "" {
		s.MakerSymbol = s.Symbol
	}

	if s.HedgeSession == "" {
		s.HedgeSession = s.SourceExchange
	}

	if s.MakerSession == "" {
		s.MakerSession = s.MakerExchange
	}

	if s.SignalSource == "" {
		s.SignalSource = s.HedgeSession
	}

	return nil
}

func (s *Strategy) Validate() error {
	if s.Quantity.IsZero() && s.QuantityScale == nil {
		return errors.New("quantity or quantityScale can not be empty")
	}

	if !s.QuantityMultiplier.IsZero() && s.QuantityMultiplier.Sign() < 0 {
		return errors.New("quantityMultiplier can not be a negative number")
	}

	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	return nil
}

func (s *Strategy) quoteWorker(ctx context.Context) {
	s.wg.Add(1)
	defer s.wg.Done()

	defer close(s.quoteWorkerDoneC)

	ticker := time.NewTicker(timejitter.Milliseconds(s.UpdateInterval.Duration(), 200))
	defer ticker.Stop()

	defer func() {
		if err := s.activeMakerOrders.GracefulCancel(ctx, s.makerSession.Exchange); err != nil {
			s.logger.WithError(err).Errorf("can not cancel %s orders", s.Symbol)
		}
	}()

	for {
		select {

		case <-s.stopQuoteWorkerC:
			s.logger.Warnf("%s quote worker is exiting due to the stop signal", s.Symbol)
			return

		case <-ctx.Done():
			s.logger.Warnf("%s quote worker is exiting due to the cancelled context", s.Symbol)
			return

		case <-ticker.C:
			if err := s.updateQuote(ctx); err != nil {
				if !errors.Is(err, context.Canceled) {
					s.logger.WithError(err).Errorf("unable to place maker orders")
				}
			}

		}
	}
}

func (s *Strategy) accountUpdater(ctx context.Context) {
	s.wg.Add(1)
	defer s.wg.Done()

	ticker := time.NewTicker(3 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			if _, err := s.sourceSession.UpdateAccount(ctx); err != nil {
				s.logger.WithError(err).Errorf("unable to update account")
			}

			if err := s.accountValueCalculator.UpdatePrices(ctx); err != nil {
				s.logger.WithError(err).Errorf("unable to update account value with prices")
				return
			}

			netValue := s.accountValueCalculator.NetValue()
			s.logger.Infof("hedge session net value ~= %f USD", netValue.Float64())
		}
	}
}

func (s *Strategy) houseCleanWorker(ctx context.Context) {
	s.wg.Add(1)
	defer s.wg.Done()

	expiryDuration := 3 * time.Hour
	ticker := time.NewTicker(15 * time.Minute)

	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			s.orderStore.Prune(expiryDuration)
		}

	}
}

func (s *Strategy) getCoveredPosition() fixedpoint.Value {
	coveredPosition := s.positionExposure.pending.Get()
	coveredPositionMetrics.With(s.metricsLabels).Set(coveredPosition.Float64())
	return coveredPosition
}

func (s *Strategy) getUncoveredPosition() fixedpoint.Value {
	return s.positionExposure.GetUncovered()
}

func (s *Strategy) hedgeWorker(ctx context.Context) {
	s.wg.Add(1)
	defer s.wg.Done()

	ticker := time.NewTicker(timejitter.Milliseconds(s.HedgeInterval.Duration(), 200))
	defer ticker.Stop()

	profitChanged := false
	reportTicker := time.NewTicker(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return

		case tt := <-ticker.C:
			// For positive position and positive covered position:
			// uncover position = +5 - +3 (covered position) = 2
			//
			// For positive position and negative covered position:
			// uncover position = +5 - (-3) (covered position) = 8
			//
			// meaning we bought 5 on MAX and sent buy order with 3 on binance
			//
			// For negative position:
			// uncover position = -5 - -3 (covered position) = -2
			s.tradeCollector.Process()

			position := s.Position.GetBase()

			if position.IsZero() || s.Position.IsDust() {
				s.resetPositionStartTime()
			} else {
				s.setPositionStartTime(tt)
			}

			uncoverPosition := s.getUncoveredPosition()
			absPos := uncoverPosition.Abs()

			// TODO: consider synthetic hedge here
			if s.sourceMarket.IsDustQuantity(absPos, s.lastPrice.Get()) {
				continue
			}

			if s.DisableHedge {
				continue
			}

			s.hedge(ctx, uncoverPosition)
			profitChanged = true

		case <-reportTicker.C:
			if profitChanged {
				if s.reportProfitStatsRateLimiter.Allow() {
					bbgo.Notify(s.ProfitStats)
				}

				profitChanged = false
			}
		}
	}
}

func (s *Strategy) isSimpleHedgeMode() bool {
	return s.SourceSymbol != "" && s.SplitHedge == nil && s.SyntheticHedge == nil
}

func (s *Strategy) CrossRun(
	ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession,
) error {
	if bbgo.IsBackTesting {
		panic("xmaker strategy does not support backtesting")
	}

	s.tradingCtx, s.cancelTrading = context.WithCancel(ctx)

	instanceID := s.InstanceID()

	configWriter := bytes.NewBuffer(nil)
	s.PrintConfig(configWriter, true, false)
	s.logger.Infof("config: %s", configWriter.String())

	if err := dynamic.InitializeConfigMetrics(ID, instanceID, s); err != nil {
		return err
	}

	// configure sessions
	sourceSession, ok := sessions[s.SourceExchange]
	if !ok {
		return fmt.Errorf("source exchange session %s is not defined", s.SourceExchange)
	}

	s.sourceSession = sourceSession

	makerSession, ok := sessions[s.MakerExchange]
	if !ok {
		return fmt.Errorf("maker exchange session %s is not defined", s.MakerExchange)
	}

	s.makerSession = makerSession

	s.sourceMarket, ok = s.sourceSession.Market(s.SourceSymbol)
	if !ok {
		return fmt.Errorf("source session market %s is not defined", s.SourceSymbol)
	}

	s.makerMarket, ok = s.makerSession.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("maker session market %s is not defined", s.Symbol)
	}

	s.simpleHedgeMode = s.isSimpleHedgeMode()

	if s.UseSandbox {
		balances := types.BalanceMap{
			s.sourceMarket.BaseCurrency:  types.NewBalance(s.sourceMarket.BaseCurrency, fixedpoint.NewFromFloat(50.0)),
			s.sourceMarket.QuoteCurrency: types.NewBalance(s.sourceMarket.QuoteCurrency, fixedpoint.NewFromFloat(10_000.0)),
		}

		customBalances := s.SandboxExchangeBalances
		if customBalances != nil {
			balances = types.BalanceMap{}
			for cur, balance := range customBalances {
				balances[cur] = types.NewBalance(cur, balance)
			}
		}

		s.logger.Warnf("using sandbox exchange to simulate hedge with balances: %+v", balances)

		sandboxEx := sandbox.New(s.sourceSession.Exchange, s.sourceSession.MarketDataStream, s.SourceSymbol, balances)

		if err := sandboxEx.Initialize(ctx); err != nil {
			return err
		}

		// replace the user data stream created in the exchange session
		userDataStream := sandboxEx.GetUserDataStream()
		s.sourceSession.Exchange = sandboxEx
		s.sourceSession.UserDataStream = userDataStream

		// rebind user data connectivity
		userDataConnectivity := types.NewConnectivity()
		userDataConnectivity.Bind(userDataStream)
		s.sourceSession.UserDataConnectivity = userDataConnectivity

		// rebuild the connectivity group
		s.sourceSession.Connectivity = types.NewConnectivityGroup(s.sourceSession.MarketDataConnectivity, s.sourceSession.UserDataConnectivity)
	}

	indicators := s.sourceSession.Indicators(s.SourceSymbol)

	s.boll = indicators.BOLL(
		types.IntervalWindow{
			Interval: s.BollBandInterval,
			Window:   21,
		}, 1.0,
	)

	// restore state
	s.groupID = util.FNV32(instanceID)
	s.logger.Infof("using group id %d from fnv(%s)", s.groupID, instanceID)

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.makerMarket)
	}

	s.Position.Strategy = ID
	s.Position.StrategyInstanceID = instanceID

	if s.makerSession.MakerFeeRate.Sign() > 0 || s.makerSession.TakerFeeRate.Sign() > 0 {
		s.Position.SetExchangeFeeRate(
			types.ExchangeName(s.MakerExchange), types.ExchangeFee{
				MakerFeeRate: s.makerSession.MakerFeeRate,
				TakerFeeRate: s.makerSession.TakerFeeRate,
			},
		)
	}

	if s.sourceSession.MakerFeeRate.Sign() > 0 || s.sourceSession.TakerFeeRate.Sign() > 0 {
		s.Position.SetExchangeFeeRate(
			types.ExchangeName(s.SourceExchange), types.ExchangeFee{
				MakerFeeRate: s.sourceSession.MakerFeeRate,
				TakerFeeRate: s.sourceSession.TakerFeeRate,
			},
		)
	}

	if err := tradingutil.UniversalCancelAllOrders(ctx, s.makerSession.Exchange, s.Symbol, nil); err != nil {
		s.logger.WithError(err).Warnf("unable to cancel all orders: %v", err)
	}

	s.Position.UpdateMetrics()
	bbgo.Notify("xmaker: %s position is restored", s.Symbol, s.Position)

	if s.ProfitStats == nil {
		s.ProfitStats = &ProfitStats{
			ProfitStats:   types.NewProfitStats(s.makerMarket),
			MakerExchange: s.makerSession.ExchangeName,
		}
	}

	// initialize the price resolver
	allMarkets := s.makerSession.Markets()
	sourceMarkets := s.sourceSession.Markets()
	if len(sourceMarkets) == 0 {
		return fmt.Errorf("source exchange %s has no markets", s.SourceExchange)
	}
	allMarkets.Merge(sourceMarkets)
	s.priceSolver = pricesolver.NewSimplePriceResolver(allMarkets)
	s.priceSolver.BindStream(s.sourceSession.MarketDataStream)
	s.sourceSession.UserDataStream.OnTradeUpdate(s.priceSolver.UpdateFromTrade)

	s.accountValueCalculator = bbgo.NewAccountValueCalculator(
		s.sourceSession, s.priceSolver, s.sourceMarket.QuoteCurrency,
	)
	if err := s.accountValueCalculator.UpdatePrices(ctx); err != nil {
		return err
	}

	s.sourceSession.MarketDataStream.OnKLineClosed(
		types.KLineWith(
			s.SourceSymbol, types.Interval1m, func(k types.KLine) {
				feeToken := s.sourceSession.Exchange.PlatformFeeCurrency()
				if feePrice, ok := s.priceSolver.ResolvePrice(feeToken, feeTokenQuote); ok {
					s.Position.SetFeeAverageCost(feeToken, feePrice)
				}
			},
		),
	)

	s.makerSession.MarketDataStream.OnKLineClosed(
		types.KLineWith(
			s.Symbol, types.Interval1m, func(k types.KLine) {
				feeToken := s.makerSession.Exchange.PlatformFeeCurrency()
				if feePrice, ok := s.priceSolver.ResolvePrice(feeToken, feeTokenQuote); ok {
					s.Position.SetFeeAverageCost(feeToken, feePrice)
				}
			},
		),
	)

	if s.ProfitFixerConfig != nil {
		bbgo.Notify("Fixing %s profitStats and position...", s.Symbol)

		s.logger.Infof("profitFixer is enabled, checking checkpoint: %+v", s.ProfitFixerConfig.TradesSince)

		if s.ProfitFixerConfig.TradesSince.Time().IsZero() {
			return errors.New("tradesSince time can not be zero")
		}

		position := types.NewPositionFromMarket(s.makerMarket)
		position.ExchangeFeeRates = s.Position.ExchangeFeeRates
		position.FeeRate = s.Position.FeeRate
		position.StrategyInstanceID = s.Position.StrategyInstanceID
		position.Strategy = s.Position.Strategy

		profitStats := types.NewProfitStats(s.makerMarket)

		fixer := common.NewProfitFixer()
		// fixer.ConverterManager = s.ConverterManager

		if ss, ok := makerSession.Exchange.(types.ExchangeTradeHistoryService); ok {
			s.logger.Infof("adding makerSession %s to profitFixer", makerSession.Name)
			fixer.AddExchange(makerSession.Name, ss)
		}

		if ss, ok := sourceSession.Exchange.(types.ExchangeTradeHistoryService); ok {
			s.logger.Infof("adding hedgeSession %s to profitFixer", sourceSession.Name)
			fixer.AddExchange(sourceSession.Name, ss)
		}

		if err2 := fixer.Fix(
			ctx, s.makerMarket.Symbol,
			s.ProfitFixerConfig.TradesSince.Time(),
			time.Now(),
			profitStats,
			position,
		); err2 != nil {
			return err2
		}

		bbgo.Notify("Fixed %s position", s.Symbol, position)
		bbgo.Notify("Fixed %s profitStats", s.Symbol, profitStats)

		s.Position = position
		s.ProfitStats.ProfitStats = profitStats
	}

	if s.SpreadMaker != nil && s.SpreadMaker.Enabled {
		if err := s.SpreadMaker.Bind(s.tradingCtx, s.makerSession, s.Symbol); err != nil {
			return err
		}
	}

	if s.EnableArbitrage {
		makerMarketStream := s.makerSession.Exchange.NewStream()
		makerMarketStream.SetPublicOnly()
		makerMarketStream.Subscribe(
			types.BookChannel, s.Symbol, types.SubscribeOptions{
				Depth: types.DepthLevelFull,
				Speed: types.SpeedLow,
			},
		)

		s.makerBook = types.NewStreamBook(s.Symbol, s.makerSession.ExchangeName)
		s.makerBook.BindStream(makerMarketStream)

		if err := makerMarketStream.Connect(s.tradingCtx); err != nil {
			return err
		}
	}

	s.CircuitBreaker.OnPanic(
		func() {
			s.gracefulShutDown(context.Background())
			panic(fmt.Errorf("panic triggered the circuit breaker on %s %s", s.MakerExchange, s.Symbol))
		},
	)

	// TODO: replace this with HedgeMarket
	sourceMarketStream := s.sourceSession.Exchange.NewStream()
	sourceMarketStream.SetPublicOnly()
	sourceMarketStream.Subscribe(
		types.BookChannel, s.SourceSymbol, types.SubscribeOptions{
			Depth: types.DepthLevelFull,
			Speed: types.SpeedLow,
		},
	)

	s.sourceBook = types.NewStreamBook(s.SourceSymbol, s.sourceSession.ExchangeName)
	s.sourceBook.BindStream(sourceMarketStream)
	s.depthSourceBook = types.NewDepthBook(s.sourceBook)

	if err := sourceMarketStream.Connect(s.tradingCtx); err != nil {
		return err
	}

	if s.EnableSignalMargin {
		s.logger.Infof("signal margin is enabled")

		if s.SignalReverseSideMargin == nil || s.SignalReverseSideMargin.Scale == nil {
			return errors.New("signalReverseSideMarginScale can not be nil when signal margin is enabled")
		}

		if s.SignalTrendSideMarginDiscount == nil || s.SignalTrendSideMarginDiscount.Scale == nil {
			return errors.New("signalTrendSideMarginScale can not be nil when signal margin is enabled")
		}

		scale, err := s.SignalReverseSideMargin.Scale.Scale()
		if err != nil {
			return err
		}

		minAdditionalMargin := scale.Call(0.0)
		middleAdditionalMargin := scale.Call(1.0)
		maxAdditionalMargin := scale.Call(2.0)
		s.logger.Infof(
			"signal margin range: %.2f%% @ 0.0 ~ %.2f%% @ 1.0 ~ %.2f%% @ 2.0",
			minAdditionalMargin*100.0,
			middleAdditionalMargin*100.0,
			maxAdditionalMargin*100.0,
		)
	}

	for _, signalConfig := range s.SignalConfigList.Signals {
		sigProvider := signalConfig.Signal
		if setter, ok := sigProvider.(signal.StreamBookSetter); ok {
			s.logger.Infof("setting stream book on signal %T", sigProvider)
			setter.SetStreamBook(s.sourceBook)
		}

		// pass logger to the signal provider
		if setter, ok := sigProvider.(interface {
			SetLogger(logger logrus.FieldLogger)
		}); ok {
			setter.SetLogger(s.logger)
		}

		if binder, ok := sigProvider.(SessionBinder); ok {
			s.logger.Infof("binding session on signal %T", sigProvider)
			if err := binder.Bind(s.tradingCtx, s.sourceSession, s.SourceSymbol); err != nil {
				return err
			}
		}
	}

	s.activeMakerOrders = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeMakerOrders.BindStream(s.makerSession.UserDataStream)

	s.orderStore = core.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(s.makerSession.UserDataStream)

	s.tradeCollector = core.NewTradeCollector(s.Symbol, s.Position, s.orderStore)
	s.tradeCollector.TradeStore().SetPruneEnabled(true)

	if s.NotifyTrade {
		s.tradeCollector.OnTrade(
			func(trade types.Trade, profit, netProfit fixedpoint.Value) {
				bbgo.Notify(trade)
			},
		)
	}

	s.tradeCollector.OnTrade(
		func(trade types.Trade, profit, netProfit fixedpoint.Value) {
			delta := trade.PositionDelta()

			// for direct hedge: trades from source session are always hedge trades
			if s.simpleHedgeMode && trade.Exchange == s.sourceSession.ExchangeName {
				s.positionExposure.Close(delta)
			} else if trade.Exchange == s.makerSession.ExchangeName {
				// spread maker: trades from maker session can be hedge trades only when spread maker is enabled and it's a spread maker order
				if s.SpreadMaker != nil && s.SpreadMaker.Enabled && s.SpreadMaker.orderStore.Exists(trade.OrderID) {
					s.positionExposure.Close(delta)
				} else {
					s.positionExposure.Open(delta)
				}
			}

			s.ProfitStats.AddTrade(trade)

		},
	)

	// callbacks for RecordPosition
	s.tradeCollector.OnTrade(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
		if profit.Compare(fixedpoint.Zero) == 0 {
			s.Environment.RecordPosition(s.Position, trade, nil)
		}
	})

	s.tradeCollector.OnProfit(func(trade types.Trade, profit *types.Profit) {
		if profit == nil {
			return
		}
		s.Environment.RecordPosition(s.Position, trade, profit)
	})

	shouldNotifyProfit := func(trade types.Trade, profit *types.Profit) bool {
		amountThreshold := s.NotifyIgnoreSmallAmountProfitTrade
		if amountThreshold.IsZero() {
			return true
		} else if trade.QuoteQuantity.Sign() > 0 && trade.QuoteQuantity.Compare(amountThreshold) >= 0 {
			return true
		}

		return false
	}

	s.tradeCollector.OnProfit(
		func(trade types.Trade, profit *types.Profit) {
			// a simple guard - the behavior was changed at some timepoint
			if profit == nil {
				return
			}

			if s.CircuitBreaker != nil {
				s.CircuitBreaker.RecordProfit(profit.Profit, trade.Time.Time())
			}

			if shouldNotifyProfit(trade, profit) {
				bbgo.Notify(profit)
			}

			netProfitMarginHistogram.With(s.metricsLabels).Observe(profit.NetProfitMargin.Float64())
			s.ProfitStats.AddProfit(*profit)
		},
	)

	s.tradeCollector.OnPositionUpdate(
		func(position *types.Position) {
			bbgo.Notify(position)
		},
	)

	s.tradeCollector.OnRecover(
		func(trade types.Trade) {
			bbgo.Notify("Recovered trade", trade)
		},
	)

	// bind two user data streams so that we can collect the trades together
	s.tradeCollector.BindStream(s.makerSession.UserDataStream)

	if s.SplitHedge != nil && s.SplitHedge.Enabled {
		s.logger.Infof("split hedge is enabled: %+v", s.SplitHedge)
		if err := s.SplitHedge.InitializeAndBind(sessions, s); err != nil {
			return err
		}
	} else if s.SyntheticHedge != nil && s.SyntheticHedge.Enabled {
		s.logger.Infof("syntheticHedge is enabled: %+v", s.SyntheticHedge)
		if err := s.SyntheticHedge.InitializeAndBind(sessions, s); err != nil {
			return err
		}
	} else {
		s.orderStore.BindStream(s.sourceSession.UserDataStream)
		s.tradeCollector.BindStream(s.sourceSession.UserDataStream)
	}

	s.sourceUserDataConnectivity = s.sourceSession.UserDataConnectivity
	s.sourceMarketDataConnectivity = s.sourceSession.MarketDataConnectivity

	s.connectivityGroup = types.NewConnectivityGroup(
		s.sourceSession.UserDataConnectivity, s.makerSession.UserDataConnectivity,
	)

	go func() {
		s.logger.Infof("waiting for authentication connections to be ready...")

		select {
		case <-ctx.Done():

		case <-time.After(3 * time.Minute):
			s.connectivityGroup.DebugStates()
			s.logger.Panicf("authentication timeout, exiting...")

		case <-s.connectivityGroup.AllAuthedC(ctx):
		}

		s.logger.Infof("all user data streams are connected, starting workers...")

		if s.SplitHedge != nil && s.SplitHedge.Enabled {
			if err := s.SplitHedge.Start(s.tradingCtx); err != nil {
				s.logger.WithError(err).Errorf("failed to start split hedge")
			}
		} else if s.SyntheticHedge != nil && s.SyntheticHedge.Enabled {
			if err := s.SyntheticHedge.Start(s.tradingCtx); err != nil {
				s.logger.WithError(err).Errorf("failed to start syntheticHedge")
			}
		}

		go s.accountUpdater(s.tradingCtx)
		go s.hedgeWorker(s.tradingCtx)
		go s.quoteWorker(s.tradingCtx)
		go s.houseCleanWorker(s.tradingCtx)

		if s.RecoverTrade {
			go s.tradeRecover(s.tradingCtx)
		}

		if !s.Position.IsDust() {
			// restore position into the position exposure
			s.positionExposure.Open(s.Position.GetBase())
		}
	}()

	bbgo.OnShutdown(
		ctx, func(shutdownCtx context.Context, wg *sync.WaitGroup) {
			// defer work group done to mark the strategy as stopped
			defer wg.Done()
			s.gracefulShutDown(shutdownCtx)
		},
	)

	return nil
}

func (s *Strategy) gracefulShutDown(shutdownCtx context.Context) {
	bbgo.Notify("Shutting down %s %s", ID, s.Symbol, s.Position)

	// send stop signal to the quoteWorker
	close(s.stopQuoteWorkerC)
	<-s.quoteWorkerDoneC

	s.logger.Infof("quote worker is done, ensure that we have canceled all maker orders...")

	if s.SpreadMaker != nil && s.SpreadMaker.Enabled {
		makerOrder, err := s.SpreadMaker.cancelAndQueryOrder(shutdownCtx)
		if err != nil {
			s.logger.WithError(err).Errorf("unable to cancel spread maker orders")
		} else if makerOrder != nil {
			s.logger.Infof("spread maker orders are cancelled, current position: %s", s.Position)
		}
	}

	// make sure all orders are cancelled
	if err := tradingutil.UniversalCancelAllOrders(shutdownCtx, s.makerSession.Exchange, s.Symbol, nil); err != nil {
		s.logger.WithError(err).Errorf("graceful cancel order error")
	}

	if s.tradeCollector.Process() {
		s.logger.Infof("trade collector process found trades to process after quote worker is done")
	}

	// wait for a while to let the hedge worker to process the last position
	time.Sleep(s.HedgeInterval.Duration() * 5)

	// stop the hedge workers
	if s.SplitHedge != nil && s.SplitHedge.Enabled {
		if err := s.SplitHedge.Stop(shutdownCtx); err != nil {
			s.logger.WithError(err).Errorf("failed to stop splitHedge")
		}
	} else if s.SyntheticHedge != nil && s.SyntheticHedge.Enabled {
		if err := s.SyntheticHedge.Stop(shutdownCtx); err != nil {
			s.logger.WithError(err).Errorf("failed to stop syntheticHedge")
		}
	}

	s.cancelTrading()
	s.logger.Infof("canceling trading context and waiting for workers to stop...")
	s.wg.Wait()

	bbgo.Sync(shutdownCtx, s)
}

func isSignalSidePosition(signal float64, side types.SideType) bool {
	switch side {
	case types.SideTypeBuy:
		return signal > 0

	case types.SideTypeSell:
		return signal < 0

	}

	return false
}

func getPositionProfitPrice(side types.SideType, cost, profitRatio fixedpoint.Value) fixedpoint.Value {
	switch side {
	case types.SideTypeBuy:
		return cost.Mul(profitRatio.Add(fixedpoint.One))

	case types.SideTypeSell:
		return cost.Mul(fixedpoint.One.Sub(profitRatio))

	}

	return cost
}

func aggregatePriceVolumeSliceWithPriceFilter(
	side types.SideType,
	pvs types.PriceVolumeSlice,
	filterPrice fixedpoint.Value,
) types.PriceVolume {
	var totalVolume = fixedpoint.Zero
	var lastPrice = fixedpoint.Zero
	for _, pv := range pvs {
		if side == types.SideTypeSell && pv.Price.Compare(filterPrice) > 0 {
			break
		} else if side == types.SideTypeBuy && pv.Price.Compare(filterPrice) < 0 {
			break
		}

		lastPrice = pv.Price
		totalVolume = totalVolume.Add(pv.Volume)
	}

	return types.PriceVolume{
		Price:  lastPrice,
		Volume: totalVolume,
	}
}

func parseSymbolSelector(
	selector string, sessions map[string]*bbgo.ExchangeSession,
) (*bbgo.ExchangeSession, types.Market, error) {
	parts := strings.SplitN(selector, ".", 2)
	if len(parts) != 2 {
		return nil, types.Market{}, fmt.Errorf("invalid symbol selector: %s", selector)
	}

	sessionName, symbol := parts[0], parts[1]
	session, ok := sessions[sessionName]
	if !ok {
		return nil, types.Market{}, fmt.Errorf("session %s not found", sessionName)
	}

	market, ok := session.Market(symbol)
	if !ok {
		return nil, types.Market{}, fmt.Errorf("market %s not found in session %s", symbol, sessionName)
	}
	return session, market, nil
}
