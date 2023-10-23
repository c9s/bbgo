package trend

import (
	"github.com/c9s/bbgo/pkg/types"
)

// Double Exponential Moving Average (DEMA)

// The [Dema](https://pkg.go.dev/github.com/cinar/indicator#Dema) function calculates
// the Double Exponential Moving Average (DEMA) for a given period.

// The double exponential moving average (DEMA) is a technical indicator introduced by Patrick Mulloy.
// The purpose is to reduce the amount of noise present in price charts used by technical traders.
// The DEMA uses two exponential moving averages (EMAs) to eliminate lag.
// It helps confirm uptrends when the price is above the average, and helps confirm downtrends
// when the price is below the average. When the price crosses the average that may signal a trend change.

// ```
//
//	DEMA = (2 * EMA(values)) - EMA(EMA(values))
//
// ```

type DEMAStream struct {
	// embedded struct
	*types.Float64Series

	ema1 *EWMAStream
	ema2 *EWMAStream
}

func DEMA(source types.Float64Source, window int) *DEMAStream {
	ema1 := EWMA2(source, window)
	ema2 := EWMA2(source, window)
	return &DEMAStream{ema1: ema1, ema2: ema2}
}

func (s *DEMAStream) Calculate(v float64) float64 {
	e1 := s.ema1.Last(0)
	e2 := s.ema2.Last(0)
	dema := e1*2 - e2
	return dema
}
