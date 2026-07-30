package xmaker

import (
	"fmt"
	"math"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// AdaptiveQuoteInterval adjusts the maker quote (place/cancel) interval based on
// market volatility measured by the ATR percentage (ATRP) of the source market.
//
// When volatility is low, the interval is stretched towards MaxInterval to reduce
// the load on the exchange; when volatility is high, the interval is shortened
// towards MinInterval to react faster. The mapping is a clamped inverse linear
// scale, so the resulting interval always stays within [MinInterval, MaxInterval].
//
// This feature only controls the periodic quote ticker. It does NOT affect the
// fast-cancel path, which is driven by market-trade events independently.
type AdaptiveQuoteInterval struct {
	// Enabled enables the adaptive quote interval feature
	Enabled bool `json:"enabled"`

	// Interval is the kline interval used to compute the ATR, default 1m
	Interval types.Interval `json:"interval"`

	// Window is the ATR window, default 14
	Window int `json:"window"`

	// MinInterval is the fastest (shortest) quote interval, used at high volatility
	MinInterval types.Duration `json:"minInterval"`

	// MaxInterval is the slowest (longest) quote interval, used at low volatility
	MaxInterval types.Duration `json:"maxInterval"`

	// LowVolatility is the ATRP (atr/close) ratio at or below which MaxInterval is used
	LowVolatility fixedpoint.Value `json:"lowVolatility"`

	// HighVolatility is the ATRP (atr/close) ratio at or above which MinInterval is used
	HighVolatility fixedpoint.Value `json:"highVolatility"`

	atrp   *indicatorv2.ATRPStream
	scale  *bbgo.LinearScale
	logger logrus.FieldLogger
}

func (a *AdaptiveQuoteInterval) Defaults() error {
	if a.Interval == "" {
		a.Interval = types.Interval1m
	}

	if a.Window == 0 {
		a.Window = 14
	}

	if a.MinInterval == 0 {
		a.MinInterval = types.Duration(time.Second)
	}

	if a.MaxInterval == 0 {
		a.MaxInterval = types.Duration(time.Minute)
	}

	if a.LowVolatility.IsZero() {
		a.LowVolatility = fixedpoint.NewFromFloat(0.0005) // 0.05%
	}

	if a.HighVolatility.IsZero() {
		a.HighVolatility = fixedpoint.NewFromFloat(0.005) // 0.5%
	}

	return nil
}

func (a *AdaptiveQuoteInterval) Validate() error {
	if a.Window <= 0 {
		return fmt.Errorf("adaptiveQuoteInterval.window must be positive, got %d", a.Window)
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

// Bind sets up the ATRP indicator on the given session/symbol and builds the
// inverse linear scale that maps the volatility ratio to a quote interval.
func (a *AdaptiveQuoteInterval) Bind(session *bbgo.ExchangeSession, symbol string, logger logrus.FieldLogger) {
	a.logger = logger.WithField("feature", "adaptiveQuoteInterval")
	a.atrp = session.Indicators(symbol).ATRP(a.Interval, a.Window)
	a.scale = a.buildScale()
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

// intervalForVolatility maps an ATRP ratio to a quote interval. It is a pure
// function of the configured scale and is unit-testable without a live session.
// A non-positive atrp (indicator warm-up) falls back to the given duration.
func (a *AdaptiveQuoteInterval) intervalForVolatility(atrp float64, fallback time.Duration) time.Duration {
	if atrp <= 0 {
		return fallback
	}

	ms := a.scale.Call(atrp)
	// Round rather than truncate: the domain endpoints go through a
	// fixedpoint->float64 conversion (exact under stdlib, slightly lossy under
	// dnum), so a boundary volatility can yield e.g. 999.999ms which must map
	// back to the intended 1s bound.
	return time.Duration(math.Round(ms)) * time.Millisecond
}

// NextInterval reads the current volatility from the ATRP indicator and returns
// the corresponding quote interval, falling back to the given duration while the
// indicator is still warming up.
func (a *AdaptiveQuoteInterval) NextInterval(fallback time.Duration) time.Duration {
	atrp := a.atrp.Last(0)
	interval := a.intervalForVolatility(atrp, fallback)

	if a.logger != nil {
		a.logger.Debugf("adaptive quote interval: atrp=%f -> %s", atrp, interval)
	}

	return interval
}
