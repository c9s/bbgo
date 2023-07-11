package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Commodity Channel Index
// Refer URL: http://www.andrewshamlet.net/2017/07/08/python-tutorial-cci
// with modification of ddof=0 to let standard deviation to be divided by N instead of N-1
//
// CCI = (Typical Price  -  n-period SMA of TP) / (Constant x Mean Deviation)
//
// Typical Price (TP) = (High + Low + Close)/3
//
// Constant = .015
//
// The Commodity Channel Index (CCI) is a technical analysis indicator that is used to identify potential overbought or oversold conditions
// in a security's price. It was originally developed for use in commodity markets, but can be applied to any security that has a sufficient
// amount of price data. The CCI is calculated by taking the difference between the security's typical price (the average of its high, low, and
// closing prices) and its moving average, and then dividing the result by the mean absolute deviation of the typical price. This resulting value
// is then plotted as a line on the price chart, with values above +100 indicating overbought conditions and values below -100 indicating
// oversold conditions. The CCI can be used by traders to identify potential entry and exit points for trades, or to confirm other technical
// analysis signals.

type CCIStream struct {
	*types.Float64Series

	TypicalPrice *types.Float64Series

	source types.Float64Source
	window int
}

func CCI(source types.Float64Source, window int) *CCIStream {
	s := &CCIStream{
		Float64Series: types.NewFloat64Series(),
		TypicalPrice:  types.NewFloat64Series(),
		source:        source,
		window:        window,
	}
	s.Bind(source, s)
	return s
}

func (s *CCIStream) Calculate(value float64) float64 {
	var tp = value
	if s.TypicalPrice.Length() > 0 {
		tp = s.TypicalPrice.Last(0) - s.source.Last(s.window) + value
	}

	s.TypicalPrice.Push(tp)

	ma := tp / float64(s.window)
	md := 0.
	for i := 0; i < s.window; i++ {
		diff := s.source.Last(i) - ma
		md += diff * diff
	}

	md = math.Sqrt(md / float64(s.window))
	cci := (value - ma) / (0.015 * md)
	return cci
}
