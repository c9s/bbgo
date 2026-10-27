package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Commodity Channel Index
// Refer URL: http://www.andrewshamlet.net/2017/07/08/python-tutorial-cci
// Refer URL: https://school.stockcharts.com/doku.php?id=technical_indicators:commodity_channel_index_cci
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
//
// CCI = (x - SMA(x, window)) / (0.015 * mean absolute deviation of x from the SMA)
//
// The source is usually the typical price, e.g. CCI(HLC3(kLines), 20).
// The first value is emitted once `window` values have been received.
type CCIStream struct {
	*types.Float64Series

	values *types.Queue
	window int
}

func CCI(source types.Float64Source, window int) *CCIStream {
	checkWindow(window)

	s := &CCIStream{
		Float64Series: types.NewFloat64Series(),
		values:        types.NewQueue(window),
		window:        window,
	}

	if source != nil {
		s.Subscribe(source, func(v float64) {
			if cci, ok := s.calculate(v); ok {
				s.PushAndEmit(cci)
				s.Truncate()
			}
		})
	}

	return s
}

func (s *CCIStream) calculate(value float64) (float64, bool) {
	s.values.Update(value)
	if s.values.Length() < s.window {
		return 0, false
	}

	n := float64(s.window)
	ma := s.values.Mean(s.window)

	md := 0.
	for i := 0; i < s.window; i++ {
		md += math.Abs(s.values.Last(i) - ma)
	}
	md /= n

	if md == 0 {
		return 0, true
	}

	return (value - ma) / (0.015 * md), true
}

func (s *CCIStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}
