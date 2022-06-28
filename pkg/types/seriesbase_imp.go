package types

func (s *SeriesBase) Index(i int) float64 {
	return s.index(i)
}

func (s *SeriesBase) Last() float64 {
	return s.last()
}

func (s *SeriesBase) Length() int {
	return s.length()
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

func (s *SeriesBase) ToArray(limit ...int) (result []float64) {
	return ToArray(s, limit...)
}

func (s *SeriesBase) ToReverseArray(limit ...int) (result Float64Slice) {
	return ToReverseArray(s, limit...)
}

func (s *SeriesBase) Change(offset ...int) SeriesExtend {
	return Change(s, offset...)
}

func (s *SeriesBase) Stdev(length int) float64 {
	return Stdev(s, length)
}
