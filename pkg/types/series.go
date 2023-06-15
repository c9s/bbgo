package types

import (
	"reflect"

	"github.com/c9s/bbgo/pkg/datatype/floats"
)

// The interface maps to pinescript basic type `series`
// Access the internal historical data from the latest to the oldest
// Index(0) always maps to Last()
type Series interface {
	Last(i int) float64
	Index(i int) float64
	Length() int
}

type SeriesExtend interface {
	Series
	Sum(limit ...int) float64
	Mean(limit ...int) float64
	Abs() SeriesExtend
	Predict(lookback int, offset ...int) float64
	NextCross(b Series, lookback int) (int, float64, bool)
	CrossOver(b Series) BoolSeries
	CrossUnder(b Series) BoolSeries
	Highest(lookback int) float64
	Lowest(lookback int) float64
	Add(b interface{}) SeriesExtend
	Minus(b interface{}) SeriesExtend
	Div(b interface{}) SeriesExtend
	Mul(b interface{}) SeriesExtend
	Dot(b interface{}, limit ...int) float64
	Array(limit ...int) (result []float64)
	Reverse(limit ...int) (result floats.Slice)
	Change(offset ...int) SeriesExtend
	PercentageChange(offset ...int) SeriesExtend
	Stdev(params ...int) float64
	Rolling(window int) *RollingResult
	Shift(offset int) SeriesExtend
	Skew(length int) float64
	Variance(length int) float64
	Covariance(b Series, length int) float64
	Correlation(b Series, length int, method ...CorrFunc) float64
	AutoCorrelation(length int, lag ...int) float64
	Rank(length int) SeriesExtend
	Sigmoid() SeriesExtend
	Softmax(window int) SeriesExtend
	Entropy(window int) float64
	CrossEntropy(b Series, window int) float64
	Filter(b func(i int, value float64) bool, length int) SeriesExtend
}

func NewSeries(a Series) SeriesExtend {
	return &SeriesBase{
		Series: a,
	}
}

type UpdatableSeries interface {
	Series
	Update(float64)
}

type UpdatableSeriesExtend interface {
	SeriesExtend
	Update(float64)
}

func Clone(u UpdatableSeriesExtend) UpdatableSeriesExtend {
	method, ok := reflect.TypeOf(u).MethodByName("Clone")
	if ok {
		out := method.Func.Call([]reflect.Value{reflect.ValueOf(u)})
		return out[0].Interface().(UpdatableSeriesExtend)
	}
	panic("method Clone not exist")
}

func TestUpdate(u UpdatableSeriesExtend, input float64) UpdatableSeriesExtend {
	method, ok := reflect.TypeOf(u).MethodByName("TestUpdate")
	if ok {
		out := method.Func.Call([]reflect.Value{reflect.ValueOf(u), reflect.ValueOf(input)})
		return out[0].Interface().(UpdatableSeriesExtend)
	}
	panic("method TestUpdate not exist")
}

// The interface maps to pinescript basic type `series` for bool type
// Access the internal historical data from the latest to the oldest
// Index(0) always maps to Last()
type BoolSeries interface {
	Last() bool
	Index(int) bool
	Length() int
}

// Calculate sum of the series
// if limit is given, will only sum first limit numbers (a.Index[0..limit])
// otherwise will sum all elements
func Sum(a Series, limit ...int) (sum float64) {
	l := a.Length()
	if len(limit) > 0 && limit[0] < l {
		l = limit[0]
	}
	for i := 0; i < l; i++ {
		sum += a.Last(i)
	}
	return sum
}

// Calculate the average value of the series
// if limit is given, will only calculate the average of first limit numbers (a.Index[0..limit])
// otherwise will operate on all elements
func Mean(a Series, limit ...int) (mean float64) {
	l := a.Length()
	if l == 0 {
		return 0
	}
	if len(limit) > 0 && limit[0] < l {
		l = limit[0]
	}
	return Sum(a, l) / float64(l)
}
