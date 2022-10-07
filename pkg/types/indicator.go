package types

import (
	"fmt"
	"math"
	"reflect"
	"time"

	"github.com/wcharczuk/go-chart/v2"
	"gonum.org/v1/gonum/stat"

	"github.com/c9s/bbgo/pkg/datatype/floats"
)

// Super basic Series type that simply holds the float64 data
// with size limit (the only difference compare to float64slice)
type Queue struct {
	SeriesBase
	arr  []float64
	size int
}

func NewQueue(size int) *Queue {
	out := &Queue{
		arr:  make([]float64, 0, size),
		size: size,
	}
	out.SeriesBase.Series = out
	return out
}

func (inc *Queue) Last() float64 {
	if len(inc.arr) == 0 {
		return 0
	}
	return inc.arr[len(inc.arr)-1]
}

func (inc *Queue) Index(i int) float64 {
	if len(inc.arr)-i-1 < 0 {
		return 0
	}
	return inc.arr[len(inc.arr)-i-1]
}

func (inc *Queue) Length() int {
	return len(inc.arr)
}

func (inc *Queue) Clone() *Queue {
	out := &Queue{
		arr:  inc.arr[:],
		size: inc.size,
	}
	out.SeriesBase.Series = out
	return out
}

func (inc *Queue) Update(v float64) {
	inc.arr = append(inc.arr, v)
	if len(inc.arr) > inc.size {
		inc.arr = inc.arr[len(inc.arr)-inc.size:]
	}
}

var _ UpdatableSeriesExtend = &Queue{}

// Float64Indicator is the indicators (SMA and EWMA) that we want to use are returning float64 data.
type Float64Indicator interface {
	Last() float64
}

// The interface maps to pinescript basic type `series`
// Access the internal historical data from the latest to the oldest
// Index(0) always maps to Last()
type Series interface {
	Last() float64
	Index(int) float64
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

type SeriesBase struct {
	Series
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
		sum += a.Index(i)
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

type AbsResult struct {
	a Series
}

func (a *AbsResult) Last() float64 {
	return math.Abs(a.a.Last())
}

func (a *AbsResult) Index(i int) float64 {
	return math.Abs(a.a.Index(i))
}

func (a *AbsResult) Length() int {
	return a.a.Length()
}

// Return series that having all the elements positive
func Abs(a Series) SeriesExtend {
	return NewSeries(&AbsResult{a})
}

var _ Series = &AbsResult{}

func LinearRegression(a Series, lookback int) (alpha float64, beta float64) {
	if a.Length() < lookback {
		lookback = a.Length()
	}
	x := make([]float64, lookback)
	y := make([]float64, lookback)
	var weights []float64
	for i := 0; i < lookback; i++ {
		x[i] = float64(i)
		y[i] = a.Index(i)
	}
	alpha, beta = stat.LinearRegression(x, y, weights, false)
	return
}

func Predict(a Series, lookback int, offset ...int) float64 {
	alpha, beta := LinearRegression(a, lookback)
	o := -1.0
	if len(offset) > 0 {
		o = -float64(offset[0])
	}
	return alpha + beta*o
}

// This will make prediction using Linear Regression to get the next cross point
// Return (offset from latest, crossed value, could cross)
// offset from latest should always be positive
// lookback param is to use at most `lookback` points to determine linear regression functions
//
// You may also refer to excel's FORECAST function
func NextCross(a Series, b Series, lookback int) (int, float64, bool) {
	if a.Length() < lookback {
		lookback = a.Length()
	}
	if b.Length() < lookback {
		lookback = b.Length()
	}
	x := make([]float64, lookback)
	y1 := make([]float64, lookback)
	y2 := make([]float64, lookback)
	var weights []float64
	for i := 0; i < lookback; i++ {
		x[i] = float64(i)
		y1[i] = a.Index(i)
		y2[i] = b.Index(i)
	}
	alpha1, beta1 := stat.LinearRegression(x, y1, weights, false)
	alpha2, beta2 := stat.LinearRegression(x, y2, weights, false)
	if beta2 == beta1 {
		return 0, 0, false
	}
	indexf := (alpha1 - alpha2) / (beta2 - beta1)

	// crossed in different direction
	if indexf >= 0 {
		return 0, 0, false
	}
	return int(math.Ceil(-indexf)), alpha1 + beta1*indexf, true
}

// The result structure that maps to the crossing result of `CrossOver` and `CrossUnder`
// Accessible through BoolSeries interface
type CrossResult struct {
	a      Series
	b      Series
	isOver bool
}

func (c *CrossResult) Last() bool {
	if c.Length() == 0 {
		return false
	}
	if c.isOver {
		return c.a.Last()-c.b.Last() > 0 && c.a.Index(1)-c.b.Index(1) < 0
	} else {
		return c.a.Last()-c.b.Last() < 0 && c.a.Index(1)-c.b.Index(1) > 0
	}
}

func (c *CrossResult) Index(i int) bool {
	if i >= c.Length() {
		return false
	}
	if c.isOver {
		return c.a.Index(i)-c.b.Index(i) > 0 && c.a.Index(i+1)-c.b.Index(i+1) < 0
	} else {
		return c.a.Index(i)-c.b.Index(i) < 0 && c.a.Index(i+1)-c.b.Index(i+1) > 0
	}
}

func (c *CrossResult) Length() int {
	la := c.a.Length()
	lb := c.b.Length()
	if la > lb {
		return lb
	}
	return la
}

// a series cross above b series.
// If in current KLine, a is higher than b, and in previous KLine, a is lower than b, then return true.
// Otherwise return false.
// If accessing index <= length, will always return false
func CrossOver(a Series, b Series) BoolSeries {
	return &CrossResult{a, b, true}
}

// a series cross under b series.
// If in current KLine, a is lower than b, and in previous KLine, a is higher than b, then return true.
// Otherwise return false.
// If accessing index <= length, will always return false
func CrossUnder(a Series, b Series) BoolSeries {
	return &CrossResult{a, b, false}
}

func Highest(a Series, lookback int) float64 {
	if lookback > a.Length() {
		lookback = a.Length()
	}
	highest := a.Last()
	for i := 1; i < lookback; i++ {
		current := a.Index(i)
		if highest < current {
			highest = current
		}
	}
	return highest
}

func Lowest(a Series, lookback int) float64 {
	if lookback > a.Length() {
		lookback = a.Length()
	}
	lowest := a.Last()
	for i := 1; i < lookback; i++ {
		current := a.Index(i)
		if lowest > current {
			lowest = current
		}
	}
	return lowest
}

type NumberSeries float64

func (a NumberSeries) Last() float64 {
	return float64(a)
}

func (a NumberSeries) Index(_ int) float64 {
	return float64(a)
}

func (a NumberSeries) Length() int {
	return math.MaxInt32
}

func (a NumberSeries) Clone() NumberSeries {
	return a
}

var _ Series = NumberSeries(0)

type AddSeriesResult struct {
	a Series
	b Series
}

// Add two series, result[i] = a[i] + b[i]
func Add(a interface{}, b interface{}) SeriesExtend {
	var aa Series
	var bb Series

	switch tp := a.(type) {
	case float64:
		aa = NumberSeries(tp)
	case Series:
		aa = tp
	default:
		panic("input should be either *Series or float64")

	}
	switch tp := b.(type) {
	case float64:
		bb = NumberSeries(tp)
	case Series:
		bb = tp
	default:
		panic("input should be either *Series or float64")

	}
	return NewSeries(&AddSeriesResult{aa, bb})
}

func (a *AddSeriesResult) Last() float64 {
	return a.a.Last() + a.b.Last()
}

func (a *AddSeriesResult) Index(i int) float64 {
	return a.a.Index(i) + a.b.Index(i)
}

func (a *AddSeriesResult) Length() int {
	lengtha := a.a.Length()
	lengthb := a.b.Length()
	if lengtha < lengthb {
		return lengtha
	}
	return lengthb
}

var _ Series = &AddSeriesResult{}

type MinusSeriesResult struct {
	a Series
	b Series
}

// Minus two series, result[i] = a[i] - b[i]
func Minus(a interface{}, b interface{}) SeriesExtend {
	aa := switchIface(a)
	bb := switchIface(b)
	return NewSeries(&MinusSeriesResult{aa, bb})
}

func (a *MinusSeriesResult) Last() float64 {
	return a.a.Last() - a.b.Last()
}

func (a *MinusSeriesResult) Index(i int) float64 {
	return a.a.Index(i) - a.b.Index(i)
}

func (a *MinusSeriesResult) Length() int {
	lengtha := a.a.Length()
	lengthb := a.b.Length()
	if lengtha < lengthb {
		return lengtha
	}
	return lengthb
}

var _ Series = &MinusSeriesResult{}

func switchIface(b interface{}) Series {
	switch tp := b.(type) {
	case float64:
		return NumberSeries(tp)
	case int32:
		return NumberSeries(float64(tp))
	case int64:
		return NumberSeries(float64(tp))
	case float32:
		return NumberSeries(float64(tp))
	case int:
		return NumberSeries(float64(tp))
	case Series:
		return tp
	default:
		fmt.Println(reflect.TypeOf(b))
		panic("input should be either *Series or float64")

	}
}

// Divid two series, result[i] = a[i] / b[i]
func Div(a interface{}, b interface{}) SeriesExtend {
	aa := switchIface(a)
	if 0 == b {
		panic("Divid by zero exception")
	}
	bb := switchIface(b)
	return NewSeries(&DivSeriesResult{aa, bb})

}

type DivSeriesResult struct {
	a Series
	b Series
}

func (a *DivSeriesResult) Last() float64 {
	return a.a.Last() / a.b.Last()
}

func (a *DivSeriesResult) Index(i int) float64 {
	return a.a.Index(i) / a.b.Index(i)
}

func (a *DivSeriesResult) Length() int {
	lengtha := a.a.Length()
	lengthb := a.b.Length()
	if lengtha < lengthb {
		return lengtha
	}
	return lengthb
}

var _ Series = &DivSeriesResult{}

// Multiple two series, result[i] = a[i] * b[i]
func Mul(a interface{}, b interface{}) SeriesExtend {
	var aa Series
	var bb Series

	switch tp := a.(type) {
	case float64:
		aa = NumberSeries(tp)
	case Series:
		aa = tp
	default:
		panic("input should be either Series or float64")
	}
	switch tp := b.(type) {
	case float64:
		bb = NumberSeries(tp)
	case Series:
		bb = tp
	default:
		panic("input should be either Series or float64")

	}
	return NewSeries(&MulSeriesResult{aa, bb})

}

type MulSeriesResult struct {
	a Series
	b Series
}

func (a *MulSeriesResult) Last() float64 {
	return a.a.Last() * a.b.Last()
}

func (a *MulSeriesResult) Index(i int) float64 {
	return a.a.Index(i) * a.b.Index(i)
}

func (a *MulSeriesResult) Length() int {
	lengtha := a.a.Length()
	lengthb := a.b.Length()
	if lengtha < lengthb {
		return lengtha
	}
	return lengthb
}

var _ Series = &MulSeriesResult{}

// Calculate (a dot b).
// if limit is given, will only calculate the first limit numbers (a.Index[0..limit])
// otherwise will operate on all elements
func Dot(a interface{}, b interface{}, limit ...int) float64 {
	var aaf float64
	var aas Series
	var bbf float64
	var bbs Series
	var isaf, isbf bool

	switch tp := a.(type) {
	case float64:
		aaf = tp
		isaf = true
	case Series:
		aas = tp
		isaf = false
	default:
		panic("input should be either Series or float64")
	}
	switch tp := b.(type) {
	case float64:
		bbf = tp
		isbf = true
	case Series:
		bbs = tp
		isbf = false
	default:
		panic("input should be either Series or float64")

	}
	l := 1
	if len(limit) > 0 {
		l = limit[0]
	} else if isaf && isbf {
		l = 1
	} else {
		if !isaf {
			l = aas.Length()
		}
		if !isbf {
			if l > bbs.Length() {
				l = bbs.Length()
			}
		}
	}
	if isaf && isbf {
		return aaf * bbf * float64(l)
	} else if isaf && !isbf {
		sum := 0.
		for i := 0; i < l; i++ {
			sum += aaf * bbs.Index(i)
		}
		return sum
	} else if !isaf && isbf {
		sum := 0.
		for i := 0; i < l; i++ {
			sum += aas.Index(i) * bbf
		}
		return sum
	} else {
		sum := 0.
		for i := 0; i < l; i++ {
			sum += aas.Index(i) * bbs.Index(i)
		}
		return sum
	}
}

// Extract elements from the Series to a float64 array, following the order of Index(0..limit)
// if limit is given, will only take the first limit numbers (a.Index[0..limit])
// otherwise will operate on all elements
func Array(a Series, limit ...int) (result []float64) {
	l := a.Length()
	if len(limit) > 0 && l > limit[0] {
		l = limit[0]
	}
	if l > a.Length() {
		l = a.Length()
	}
	result = make([]float64, l)
	for i := 0; i < l; i++ {
		result[i] = a.Index(i)
	}
	return
}

// Similar to Array but in reverse order.
// Useful when you want to cache series' calculated result as float64 array
// the then reuse the result in multiple places (so that no recalculation will be triggered)
//
// notice that the return type is a Float64Slice, which implements the Series interface
func Reverse(a Series, limit ...int) (result floats.Slice) {
	l := a.Length()
	if len(limit) > 0 && l > limit[0] {
		l = limit[0]
	}
	result = make([]float64, l)
	for i := 0; i < l; i++ {
		result[l-i-1] = a.Index(i)
	}
	return
}

type ChangeResult struct {
	a      Series
	offset int
}

func (c *ChangeResult) Last() float64 {
	if c.offset >= c.a.Length() {
		return 0
	}
	return c.a.Last() - c.a.Index(c.offset)
}

func (c *ChangeResult) Index(i int) float64 {
	if i+c.offset >= c.a.Length() {
		return 0
	}
	return c.a.Index(i) - c.a.Index(i+c.offset)
}

func (c *ChangeResult) Length() int {
	length := c.a.Length()
	if length >= c.offset {
		return length - c.offset
	}
	return 0
}

// Difference between current value and previous, a - a[offset]
// offset: if not given, offset is 1.
func Change(a Series, offset ...int) SeriesExtend {
	o := 1
	if len(offset) > 0 {
		o = offset[0]
	}

	return NewSeries(&ChangeResult{a, o})
}

type PercentageChangeResult struct {
	a      Series
	offset int
}

func (c *PercentageChangeResult) Last() float64 {
	if c.offset >= c.a.Length() {
		return 0
	}
	return c.a.Last()/c.a.Index(c.offset) - 1
}

func (c *PercentageChangeResult) Index(i int) float64 {
	if i+c.offset >= c.a.Length() {
		return 0
	}
	return c.a.Index(i)/c.a.Index(i+c.offset) - 1
}

func (c *PercentageChangeResult) Length() int {
	length := c.a.Length()
	if length >= c.offset {
		return length - c.offset
	}
	return 0
}

// Percentage change between current and a prior element, a / a[offset] - 1.
// offset: if not give, offset is 1.
func PercentageChange(a Series, offset ...int) SeriesExtend {
	o := 1
	if len(offset) > 0 {
		o = offset[0]
	}

	return NewSeries(&PercentageChangeResult{a, o})
}

func Stdev(a Series, params ...int) float64 {
	length := a.Length()
	if length == 0 {
		return 0
	}
	if len(params) > 0 && params[0] < length {
		length = params[0]
	}
	ddof := 0
	if len(params) > 1 {
		ddof = params[1]
	}
	avg := Mean(a, length)
	s := .0
	for i := 0; i < length; i++ {
		diff := a.Index(i) - avg
		s += diff * diff
	}
	if length-ddof == 0 {
		return 0
	}
	return math.Sqrt(s / float64(length-ddof))
}

type CorrFunc func(Series, Series, int) float64

func Kendall(a, b Series, length int) float64 {
	if a.Length() < length {
		length = a.Length()
	}
	if b.Length() < length {
		length = b.Length()
	}
	aRanks := Rank(a, length)
	bRanks := Rank(b, length)
	concordant, discordant := 0, 0
	for i := 0; i < length; i++ {
		for j := i + 1; j < length; j++ {
			value := (aRanks.Index(i) - aRanks.Index(j)) * (bRanks.Index(i) - bRanks.Index(j))
			if value > 0 {
				concordant++
			} else {
				discordant++
			}
		}
	}
	return float64(concordant-discordant) * 2.0 / float64(length*(length-1))
}

func Rank(a Series, length int) SeriesExtend {
	if length > a.Length() {
		length = a.Length()
	}
	rank := make([]float64, length)
	mapper := make([]float64, length+1)
	for i := length - 1; i >= 0; i-- {
		ii := a.Index(i)
		counter := 0.
		for j := 0; j < length; j++ {
			if a.Index(j) <= ii {
				counter += 1.
			}
		}
		rank[i] = counter
		mapper[int(counter)] += 1.
	}
	output := NewQueue(length)
	for i := length - 1; i >= 0; i-- {
		output.Update(rank[i] - (mapper[int(rank[i])]-1.)/2)
	}
	return output
}

func Pearson(a, b Series, length int) float64 {
	if a.Length() < length {
		length = a.Length()
	}
	if b.Length() < length {
		length = b.Length()
	}
	x := make([]float64, length)
	y := make([]float64, length)
	for i := 0; i < length; i++ {
		x[i] = a.Index(i)
		y[i] = b.Index(i)
	}
	return stat.Correlation(x, y, nil)
}

func Spearman(a, b Series, length int) float64 {
	if a.Length() < length {
		length = a.Length()
	}
	if b.Length() < length {
		length = b.Length()
	}
	aRank := Rank(a, length)
	bRank := Rank(b, length)
	return Pearson(aRank, bRank, length)
}

// similar to pandas.Series.corr() function.
//
// method could either be `types.Pearson`, `types.Spearman` or `types.Kendall`
func Correlation(a Series, b Series, length int, method ...CorrFunc) float64 {
	var runner CorrFunc
	if len(method) == 0 {
		runner = Pearson
	} else {
		runner = method[0]
	}
	return runner(a, b, length)
}

// similar to pandas.Series.autocorr() function.
//
// The method computes the Pearson correlation between Series and shifted itself
func AutoCorrelation(a Series, length int, lags ...int) float64 {
	lag := 1
	if len(lags) > 0 {
		lag = lags[0]
	}
	return Pearson(a, Shift(a, lag), length)
}

// similar to pandas.Series.cov() function with ddof=0
//
// Compute covariance with Series
func Covariance(a Series, b Series, length int) float64 {
	if a.Length() < length {
		length = a.Length()
	}
	if b.Length() < length {
		length = b.Length()
	}

	meana := Mean(a, length)
	meanb := Mean(b, length)
	sum := 0.0
	for i := 0; i < length; i++ {
		sum += (a.Index(i) - meana) * (b.Index(i) - meanb)
	}
	sum /= float64(length)
	return sum
}

func Variance(a Series, length int) float64 {
	return Covariance(a, a, length)
}

// similar to pandas.Series.skew() function.
//
// Return unbiased skew over input series
func Skew(a Series, length int) float64 {
	if length > a.Length() {
		length = a.Length()
	}
	mean := Mean(a, length)
	sum2 := 0.0
	sum3 := 0.0
	for i := 0; i < length; i++ {
		diff := a.Index(i) - mean
		sum2 += diff * diff
		sum3 += diff * diff * diff
	}
	if length <= 2 || sum2 == 0 {
		return math.NaN()
	}
	l := float64(length)
	return l * math.Sqrt(l-1) / (l - 2) * sum3 / math.Pow(sum2, 1.5)
}

type ShiftResult struct {
	a      Series
	offset int
}

func (inc *ShiftResult) Last() float64 {
	if inc.offset < 0 {
		return 0
	}
	if inc.offset > inc.a.Length() {
		return 0
	}
	return inc.a.Index(inc.offset)
}
func (inc *ShiftResult) Index(i int) float64 {
	if inc.offset+i < 0 {
		return 0
	}
	if inc.offset+i > inc.a.Length() {
		return 0
	}
	return inc.a.Index(inc.offset + i)
}

func (inc *ShiftResult) Length() int {
	return inc.a.Length() - inc.offset
}

func Shift(a Series, offset int) SeriesExtend {
	return NewSeries(&ShiftResult{a, offset})
}

type RollingResult struct {
	a      Series
	window int
}

type SliceView struct {
	a      Series
	start  int
	length int
}

func (s *SliceView) Last() float64 {
	return s.a.Index(s.start)
}
func (s *SliceView) Index(i int) float64 {
	if i >= s.length {
		return 0
	}
	return s.a.Index(i + s.start)
}

func (s *SliceView) Length() int {
	return s.length
}

var _ Series = &SliceView{}

func (r *RollingResult) Last() SeriesExtend {
	return NewSeries(&SliceView{r.a, 0, r.window})
}

func (r *RollingResult) Index(i int) SeriesExtend {
	if i*r.window > r.a.Length() {
		return nil
	}
	return NewSeries(&SliceView{r.a, i * r.window, r.window})
}

func (r *RollingResult) Length() int {
	mod := r.a.Length() % r.window
	if mod > 0 {
		return r.a.Length()/r.window + 1
	} else {
		return r.a.Length() / r.window
	}
}

func Rolling(a Series, window int) *RollingResult {
	return &RollingResult{a, window}
}

type SigmoidResult struct {
	a Series
}

func (s *SigmoidResult) Last() float64 {
	return 1. / (1. + math.Exp(-s.a.Last()))
}

func (s *SigmoidResult) Index(i int) float64 {
	return 1. / (1. + math.Exp(-s.a.Index(i)))
}

func (s *SigmoidResult) Length() int {
	return s.a.Length()
}

// Sigmoid returns the input values in range of -1 to 1
// along the sigmoid or s-shaped curve.
// Commonly used in machine learning while training neural networks
// as an activation function.
func Sigmoid(a Series) SeriesExtend {
	return NewSeries(&SigmoidResult{a})
}

// SoftMax returns the input value in the range of 0 to 1
// with sum of all the probabilities being equal to one.
// It is commonly used in machine learning neural networks.
// Will return Softmax SeriesExtend result based in latest [window] numbers from [a] Series
func Softmax(a Series, window int) SeriesExtend {
	s := 0.0
	max := Highest(a, window)
	for i := 0; i < window; i++ {
		s += math.Exp(a.Index(i) - max)
	}
	out := NewQueue(window)
	for i := window - 1; i >= 0; i-- {
		out.Update(math.Exp(a.Index(i)-max) / s)
	}
	return out
}

// Entropy computes the Shannon entropy of a distribution or the distance between
// two distributions. The natural logarithm is used.
// - sum(v * ln(v))
func Entropy(a Series, window int) (e float64) {
	for i := 0; i < window; i++ {
		v := a.Index(i)
		if v != 0 {
			e -= v * math.Log(v)
		}
	}
	return e
}

// CrossEntropy computes the cross-entropy between the two distributions
func CrossEntropy(a, b Series, window int) (e float64) {
	for i := 0; i < window; i++ {
		v := a.Index(i)
		if v != 0 {
			e -= v * math.Log(b.Index(i))
		}
	}
	return e
}

func sigmoid(z float64) float64 {
	return 1. / (1. + math.Exp(-z))
}

func propagate(w []float64, gradient float64, x [][]float64, y []float64) (float64, []float64, float64) {
	logloss_epoch := 0.0
	var activations []float64
	var dw []float64
	m := len(y)
	db := 0.0
	for i, xx := range x {
		result := 0.0
		for j, ww := range w {
			result += ww * xx[j]
		}
		a := sigmoid(result + gradient)
		activations = append(activations, a)
		logloss := a*math.Log1p(y[i]) + (1.-a)*math.Log1p(1-y[i])
		logloss_epoch += logloss

		db += a - y[i]
	}
	for j := range w {
		err := 0.0
		for i, xx := range x {
			err_i := activations[i] - y[i]
			err += err_i * xx[j]
		}
		err /= float64(m)
		dw = append(dw, err)
	}

	cost := -(logloss_epoch / float64(len(x)))
	db /= float64(m)
	return cost, dw, db
}

func LogisticRegression(x []Series, y Series, lookback, iterations int, learningRate float64) *LogisticRegressionModel {
	features := len(x)
	if features == 0 {
		panic("no feature to train")
	}
	w := make([]float64, features)
	if lookback > x[0].Length() {
		lookback = x[0].Length()
	}
	xx := make([][]float64, lookback)
	for i := 0; i < lookback; i++ {
		for j := 0; j < features; j++ {
			xx[i] = append(xx[i], x[j].Index(lookback-i-1))
		}
	}
	yy := Reverse(y, lookback)

	b := 0.
	for i := 0; i < iterations; i++ {
		_, dw, db := propagate(w, b, xx, yy)
		for j := range w {
			w[j] = w[j] - (learningRate * dw[j])
		}
		b -= learningRate * db
	}
	return &LogisticRegressionModel{
		Weight:       w,
		Gradient:     b,
		LearningRate: learningRate,
	}
}

type LogisticRegressionModel struct {
	Weight       []float64
	Gradient     float64
	LearningRate float64
}

/*
// Might not be correct.
// Please double check before uncomment this
func (l *LogisticRegressionModel) Update(x []float64, y float64) {
	z := 0.0
	for i, w := l.Weight {
		z += w * x[i]
	}
	a := sigmoid(z + l.Gradient)
	//logloss := a * math.Log1p(y) + (1.-a)*math.Log1p(1-y)
	db = a - y
	var dw []float64
	for j, ww := range l.Weight {
		err := db * x[j]
		dw = append(dw, err)
	}
	for i := range l.Weight {
		l.Weight[i] -= l.LearningRate * dw[i]
	}
	l.Gradient -= l.LearningRate * db
}
*/

func (l *LogisticRegressionModel) Predict(x []float64) float64 {
	z := 0.0
	for i, w := range l.Weight {
		z += w * x[i]
	}
	return sigmoid(z + l.Gradient)
}

type Canvas struct {
	chart.Chart
	Interval Interval
}

func NewCanvas(title string, intervals ...Interval) *Canvas {
	valueFormatter := chart.TimeValueFormatter
	interval := Interval1m
	if len(intervals) > 0 {
		interval = intervals[0]
		if interval.Seconds() > 24*60*60 {
			valueFormatter = chart.TimeDateValueFormatter
		} else if interval.Seconds() > 60*60 {
			valueFormatter = chart.TimeHourValueFormatter
		} else {
			valueFormatter = chart.TimeMinuteValueFormatter
		}
	} else {
		valueFormatter = chart.IntValueFormatter
	}
	out := &Canvas{
		Chart: chart.Chart{
			Title: title,
			XAxis: chart.XAxis{
				ValueFormatter: valueFormatter,
			},
		},
		Interval: interval,
	}
	out.Chart.Elements = []chart.Renderable{
		chart.LegendLeft(&out.Chart),
	}
	return out
}

func expand(a []float64, length int, defaultVal float64) []float64 {
	l := len(a)
	if l >= length {
		return a
	}
	for i := 0; i < length-l; i++ {
		a = append([]float64{defaultVal}, a...)
	}
	return a
}

func (canvas *Canvas) Plot(tag string, a Series, endTime Time, length int, intervals ...Interval) {
	var timeline []time.Time
	e := endTime.Time()
	if a.Length() == 0 {
		return
	}
	oldest := a.Index(a.Length() - 1)
	interval := canvas.Interval
	if len(intervals) > 0 {
		interval = intervals[0]
	}
	for i := length - 1; i >= 0; i-- {
		shiftedT := e.Add(-time.Duration(i*interval.Seconds()) * time.Second)
		timeline = append(timeline, shiftedT)
	}
	canvas.Series = append(canvas.Series, chart.TimeSeries{
		Name:    tag,
		YValues: expand(Reverse(a, length), length, oldest),
		XValues: timeline,
	})
}

func (canvas *Canvas) PlotRaw(tag string, a Series, length int) {
	var x []float64
	for i := 0; i < length; i++ {
		x = append(x, float64(i))
	}
	if a.Length() == 0 {
		return
	}
	oldest := a.Index(a.Length() - 1)
	canvas.Series = append(canvas.Series, chart.ContinuousSeries{
		Name:    tag,
		XValues: x,
		YValues: expand(Reverse(a, length), length, oldest),
	})
}

// TODO: ta.linreg
