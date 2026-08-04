package xmaker

import (
	"fmt"
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// AdaptiveQuoteInterval adjusts the maker quote (place/cancel) interval based on
// market volatility measured by a RiskMetrics-style realized volatility (an EWMA
// of squared trade log-returns) on the source market-trade stream.
//
// When volatility is low, the interval is stretched towards MaxInterval to reduce
// the load on the exchange; when volatility is high, the interval is shortened
// towards MinInterval to react faster. The mapping is a clamped inverse linear
// scale, so the resulting interval always stays within [MinInterval, MaxInterval].
//
// Unlike a kline/ATR-based measure, the realized volatility updates on every trade,
// so it reacts immediately to volatility bursts and shares the same event clock as
// the fast-cancel path. This feature only controls the periodic quote ticker; it
// does NOT affect the fast-cancel path, which is driven by market-trade events
// independently.
type AdaptiveQuoteInterval struct {
	// Enabled enables the adaptive quote interval feature
	Enabled bool `json:"enabled"`

	// Lambda is the EWMA decay factor for the realized volatility (0 < λ < 1).
	// Higher means longer memory / slower reaction; the RiskMetrics default is 0.94.
	Lambda fixedpoint.Value `json:"lambda"`

	// Warmup is the minimum number of trade returns to accumulate before the
	// volatility reading is used (until then the fixed fallback interval applies).
	Warmup int `json:"warmup"`

	// MinInterval is the fastest (shortest) quote interval, used at high volatility
	MinInterval types.Duration `json:"minInterval"`

	// MaxInterval is the slowest (longest) quote interval, used at low volatility
	MaxInterval types.Duration `json:"maxInterval"`

	// LowVolatility is the realized-volatility (per-trade σ) at or below which
	// MaxInterval is used. NOTE: this is on a per-trade scale, much smaller than a
	// 1m ATRP, so it must be calibrated against the live market.
	LowVolatility fixedpoint.Value `json:"lowVolatility"`

	// HighVolatility is the realized-volatility (per-trade σ) at or above which
	// MinInterval is used. See LowVolatility on calibration.
	HighVolatility fixedpoint.Value `json:"highVolatility"`

	rv     *indicatorv2.RealizedVolatilityStream
	scale  *bbgo.LinearScale
	logger logrus.FieldLogger

	volatilityMetric      prometheus.Gauge    // set from adaptiveQuoteIntervalVolatilityMetrics
	intervalSecondsMetric prometheus.Observer // set from adaptiveQuoteIntervalSecondsMetrics
}

func (a *AdaptiveQuoteInterval) Defaults() error {
	if a.Lambda.IsZero() {
		a.Lambda = fixedpoint.NewFromFloat(0.94) // RiskMetrics default
	}

	if a.Warmup == 0 {
		a.Warmup = 30
	}

	if a.MinInterval == 0 {
		a.MinInterval = types.Duration(time.Second)
	}

	if a.MaxInterval == 0 {
		a.MaxInterval = types.Duration(time.Minute)
	}

	// NOTE: per-trade realized volatility is on a much smaller scale than a 1m
	// ATRP. These defaults are conservative placeholders and should be calibrated
	// against the live market/symbol.
	if a.LowVolatility.IsZero() {
		a.LowVolatility = fixedpoint.NewFromFloat(0.00005) // 0.005%
	}

	if a.HighVolatility.IsZero() {
		a.HighVolatility = fixedpoint.NewFromFloat(0.001) // 0.1%
	}

	return nil
}

func (a *AdaptiveQuoteInterval) Validate() error {
	if a.Lambda.Float64() <= 0 || a.Lambda.Float64() >= 1 {
		return fmt.Errorf("adaptiveQuoteInterval.lambda must be in (0, 1), got %s", a.Lambda.String())
	}

	if a.Warmup <= 0 {
		return fmt.Errorf("adaptiveQuoteInterval.warmup must be positive, got %d", a.Warmup)
	}

	if a.MinInterval <= 0 {
		return fmt.Errorf("adaptiveQuoteInterval.minInterval must be positive")
	}

	if a.MinInterval > a.MaxInterval {
		return fmt.Errorf("adaptiveQuoteInterval.minInterval (%s) must not be greater than maxInterval (%s)",
			a.MinInterval.Duration(), a.MaxInterval.Duration())
	}

	if a.LowVolatility.Compare(a.HighVolatility) >= 0 {
		return fmt.Errorf("adaptiveQuoteInterval.lowVolatility (%s) must be less than highVolatility (%s)",
			a.LowVolatility.String(), a.HighVolatility.String())
	}

	return nil
}

// Bind sets up the realized-volatility indicator on the given market-trade stream
// and builds the inverse linear scale that maps the volatility ratio to a quote
// interval. It also wires the Prometheus metric observers from the given labels.
func (a *AdaptiveQuoteInterval) Bind(
	tradeStream types.Stream, symbol string, logger logrus.FieldLogger, metricsLabels prometheus.Labels,
) {
	a.logger = logger.WithField("feature", "adaptiveQuoteInterval")
	a.rv = indicatorv2.RealizedVolatility(indicatorv2.Trades(tradeStream, symbol), a.Lambda.Float64(), a.Warmup)
	a.scale = a.buildScale()

	a.volatilityMetric = adaptiveQuoteIntervalVolatilityMetrics.With(metricsLabels)
	a.intervalSecondsMetric = adaptiveQuoteIntervalSecondsMetrics.With(metricsLabels)
}

// buildScale constructs the clamped inverse linear scale:
//
//	Domain = [lowVolatility, highVolatility] (ATRP ratio)
//	Range  = [maxInterval, minInterval] in milliseconds (descending)
//
// LinearScale.Call clamps at both domain ends, so the output is always bounded
// within [minInterval, maxInterval].
func (a *AdaptiveQuoteInterval) buildScale() *bbgo.LinearScale {
	scale := &bbgo.LinearScale{
		Domain: [2]float64{a.LowVolatility.Float64(), a.HighVolatility.Float64()},
		Range: [2]float64{
			float64(a.MaxInterval.Duration().Milliseconds()),
			float64(a.MinInterval.Duration().Milliseconds()),
		},
	}

	// LinearScale requires Solve() before Call() to compute the slope.
	_ = scale.Solve()
	return scale
}

// intervalForVolatility maps a realized-volatility reading to a quote interval. It
// is a pure function of the configured scale and is unit-testable without a live
// session. A non-positive reading (indicator warm-up) falls back to the given
// duration.
func (a *AdaptiveQuoteInterval) intervalForVolatility(vol float64, fallback time.Duration) time.Duration {
	if vol <= 0 {
		return fallback
	}

	ms := a.scale.Call(vol)
	// Round rather than truncate: the domain endpoints go through a
	// fixedpoint->float64 conversion (exact under stdlib, slightly lossy under
	// dnum), so a boundary volatility can yield e.g. 999.999ms which must map
	// back to the intended 1s bound.
	return time.Duration(math.Round(ms)) * time.Millisecond
}

// NextInterval reads the current realized volatility from the indicator and returns
// the corresponding quote interval, falling back to the given duration while the
// indicator is still warming up.
func (a *AdaptiveQuoteInterval) NextInterval(fallback time.Duration) time.Duration {
	vol := a.rv.Last(0)
	interval := a.intervalForVolatility(vol, fallback)

	// emit the raw volatility reading and the resulting interval together, so the
	// two are always recorded from the same decision. Guard against nil observers
	// because unit tests call NextInterval without Bind().
	if a.volatilityMetric != nil {
		a.volatilityMetric.Set(vol)
	}

	if a.intervalSecondsMetric != nil {
		a.intervalSecondsMetric.Observe(interval.Seconds())
	}

	if a.logger != nil {
		a.logger.Debugf("adaptive quote interval: volatility=%f -> %s", vol, interval)
	}

	return interval
}
