package pivotshort

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// MACDConfig is the MACD config of the MACD divergence detection
type MACDConfig struct {
	types.IntervalWindow // the signal window, usually 9

	// ShortPeriod is the short term period EMA, usually 12
	ShortPeriod int `json:"short"`
	// LongPeriod is the long term period EMA, usually 26
	LongPeriod int `json:"long"`
}

// newPivotLow creates the pivot low stream of the kline low prices.
// Unlike IndicatorSet.PivotLow, it respects the right window of the given IntervalWindow.
// The stream emits only when a new pivot low is found, Last(0) is the latest pivot low.
func newPivotLow(session *bbgo.ExchangeSession, symbol string, iw types.IntervalWindow) *indicatorv2.PivotLowStream {
	indicators := session.Indicators(symbol)
	if iw.RightWindow != nil {
		return indicatorv2.PivotLow(indicators.LOW(iw.Interval), iw.Window, *iw.RightWindow)
	}

	return indicators.PivotLow(iw)
}

// newPivotHigh creates the pivot high stream of the kline high prices.
// Unlike IndicatorSet.PivotHigh, it respects the right window of the given IntervalWindow.
// The stream emits only when a new pivot high is found, Last(0) is the latest pivot high.
func newPivotHigh(session *bbgo.ExchangeSession, symbol string, iw types.IntervalWindow) *indicatorv2.PivotHighStream {
	indicators := session.Indicators(symbol)
	if iw.RightWindow != nil {
		return indicatorv2.PivotHigh(indicators.HIGH(iw.Interval), iw.Window, *iw.RightWindow)
	}

	return indicators.PivotHigh(iw)
}
