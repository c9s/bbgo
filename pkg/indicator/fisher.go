package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Fisher Transform
//
// The Fisher Transform is a technical analysis indicator that is used to identify potential turning points in the price of a security.
// It is based on the idea that prices tend to be normally distributed, with most price movements being small and relatively insignificant.
// The Fisher Transform converts this normal distribution into a symmetrical, Gaussian distribution, with a peak at zero and a range of -1 to +1.
// This transformation allows for more accurate identification of price extremes, which can be used to make predictions about potential trend reversals.
// The Fisher Transform is calculated by taking the natural logarithm of the ratio of the security's current price to its moving average,
// and then double-smoothing the result. This resulting line is called the Fisher Transform line, and can be plotted on the price chart
// along with the security's price.
//
//go:generate callbackgen -type FisherTransform
type FisherTransform struct {
	types.SeriesBase
	types.IntervalWindow
	prices *types.Queue
	Values floats.Slice

	UpdateCallbacks []func(value float64)
}

func (inc *FisherTransform) Clone() types.UpdatableSeriesExtend {
	out := FisherTransform{
		IntervalWindow: inc.IntervalWindow,
		prices:         inc.prices.Clone(),
		Values:         inc.Values[:],
	}
	out.SeriesBase.Series = &out
	return &out
}

func (inc *FisherTransform) Update(value float64) {
	if inc.prices == nil {
		inc.prices = types.NewQueue(inc.Window)
		inc.SeriesBase.Series = inc
	}
	inc.prices.Update(value)
	highest := inc.prices.Highest(inc.Window)
	lowest := inc.prices.Lowest(inc.Window)
	if highest == lowest {
		inc.Values.Update(0)
		return
	}
	x := 2*((value-lowest)/(highest-lowest)) - 1
	if x == 1 {
		x = 0.9999
	} else if x == -1 {
		x = -0.9999
	}
	inc.Values.Update(0.5 * math.Log((1+x)/(1-x)))
	if len(inc.Values) > MaxNumOfEWMA {
		inc.Values = inc.Values[MaxNumOfEWMATruncateSize-1:]
	}
}

func (inc *FisherTransform) Last(i int) float64 {
	if inc.Values == nil {
		return 0.0
	}
	return inc.Values.Last(i)
}

func (inc *FisherTransform) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *FisherTransform) Length() int {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Length()
}
