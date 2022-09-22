package types

import "github.com/c9s/bbgo/pkg/datatype/floats"

func (s *SeriesBase) Index(i int) float64 {
	if s.Series == nil {
		return 0
	}
	return s.Series.Index(i)
}

func (s *SeriesBase) Last() float64 {
	if s.Series == nil {
		return 0
	}
	return s.Series.Last()
}

func (s *SeriesBase) Length() int {
	if s.Series == nil {
		return 0
	}
	return s.Series.Length()
}

func (s *SeriesBase) Sum(limit ...int) float64 {
	return Sum(s, limit...)
}

func (s *SeriesBase) Mean(limit ...int) float64 {
	return Mean(s, limit...)
}

func (s *SeriesBase) Abs() SeriesExtend {
	return Abs(s)
}

func (s *SeriesBase) Predict(lookback int, offset ...int) float64 {
	return Predict(s, lookback, offset...)
}

func (s *SeriesBase) NextCross(b Series, lookback int) (int, float64, bool) {
	return NextCross(s, b, lookback)
}

func (s *SeriesBase) CrossOver(b Series) BoolSeries {
	return CrossOver(s, b)
}

func (s *SeriesBase) CrossUnder(b Series) BoolSeries {
	return CrossUnder(s, b)
}

func (s *SeriesBase) Highest(lookback int) float64 {
	return Highest(s, lookback)
}

func (s *SeriesBase) Lowest(lookback int) float64 {
	return Lowest(s, lookback)
}

func (s *SeriesBase) Add(b interface{}) SeriesExtend {
	return Add(s, b)
}

func (s *SeriesBase) Minus(b interface{}) SeriesExtend {
	return Minus(s, b)
}

func (s *SeriesBase) Div(b interface{}) SeriesExtend {
	return Div(s, b)
}

func (s *SeriesBase) Mul(b interface{}) SeriesExtend {
	return Mul(s, b)
}

func (s *SeriesBase) Dot(b interface{}, limit ...int) float64 {
	return Dot(s, b, limit...)
}

func (s *SeriesBase) Array(limit ...int) (result []float64) {
	return Array(s, limit...)
}

func (s *SeriesBase) Reverse(limit ...int) (result floats.Slice) {
	return Reverse(s, limit...)
}

func (s *SeriesBase) Change(offset ...int) SeriesExtend {
	return Change(s, offset...)
}

func (s *SeriesBase) PercentageChange(offset ...int) SeriesExtend {
	return PercentageChange(s, offset...)
}

func (s *SeriesBase) Stdev(params ...int) float64 {
	return Stdev(s, params...)
}

func (s *SeriesBase) Rolling(window int) *RollingResult {
	return Rolling(s, window)
}

func (s *SeriesBase) Shift(offset int) SeriesExtend {
	return Shift(s, offset)
}

func (s *SeriesBase) Skew(length int) float64 {
	return Skew(s, length)
}

func (s *SeriesBase) Variance(length int) float64 {
	return Variance(s, length)
}

func (s *SeriesBase) Covariance(b Series, length int) float64 {
	return Covariance(s, b, length)
}

func (s *SeriesBase) Correlation(b Series, length int, method ...CorrFunc) float64 {
	return Correlation(s, b, length, method...)
}

func (s *SeriesBase) AutoCorrelation(length int, lag ...int) float64 {
	return AutoCorrelation(s, length, lag...)
}

func (s *SeriesBase) Rank(length int) SeriesExtend {
	return Rank(s, length)
}

func (s *SeriesBase) Sigmoid() SeriesExtend {
	return Sigmoid(s)
}

func (s *SeriesBase) Softmax(window int) SeriesExtend {
	return Softmax(s, window)
}

func (s *SeriesBase) Entropy(window int) float64 {
	return Entropy(s, window)
}

func (s *SeriesBase) CrossEntropy(b Series, window int) float64 {
	return CrossEntropy(s, b, window)
}

func (s *SeriesBase) Filter(b func(int, float64) bool, length int) SeriesExtend {
	return Filter(s, b, length)
}
