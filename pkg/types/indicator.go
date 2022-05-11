package types

import (
	"fmt"
	"math"
	"reflect"

	"gonum.org/v1/gonum/stat"
)

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
	l := -1
	if len(limit) > 0 {
		l = limit[0]
	}
	if l < a.Length() {
		l = a.Length()
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
	l := -1
	if len(limit) > 0 {
		l = limit[0]
	}
	if l < a.Length() {
		l = a.Length()
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
func Abs(a Series) Series {
	return &AbsResult{a}
}

var _ Series = &AbsResult{}

func Predict(a Series, lookback int, offset ...int) float64 {
	if a.Length() < lookback {
		lookback = a.Length()
	}
	x := make([]float64, lookback, lookback)
	y := make([]float64, lookback, lookback)
	var weights []float64
	for i := 0; i < lookback; i++ {
		x[i] = float64(i)
		y[i] = a.Index(i)
	}
	alpha, beta := stat.LinearRegression(x, y, weights, false)
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
	x := make([]float64, lookback, lookback)
	y1 := make([]float64, lookback, lookback)
	y2 := make([]float64, lookback, lookback)
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

var _ Series = NumberSeries(0)

type AddSeriesResult struct {
	a Series
	b Series
}

// Add two series, result[i] = a[i] + b[i]
func Add(a interface{}, b interface{}) Series {
	var aa Series
	var bb Series

	switch a.(type) {
	case float64:
		aa = NumberSeries(a.(float64))
	case Series:
		aa = a.(Series)
	default:
		panic("input should be either *Series or float64")

	}
	switch b.(type) {
	case float64:
		bb = NumberSeries(b.(float64))
	case Series:
		bb = b.(Series)
	default:
		panic("input should be either *Series or float64")

	}
	return &AddSeriesResult{aa, bb}
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
func Minus(a interface{}, b interface{}) Series {
	aa := switchIface(a)
	bb := switchIface(b)
	return &MinusSeriesResult{aa, bb}
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
	switch b.(type) {
	case float64:
		return NumberSeries(b.(float64))
	case int32:
		return NumberSeries(float64(b.(int32)))
	case int64:
		return NumberSeries(float64(b.(int64)))
	case float32:
		return NumberSeries(float64(b.(float32)))
	case int:
		return NumberSeries(float64(b.(int)))
	case Series:
		return b.(Series)
	default:
		fmt.Println(reflect.TypeOf(b))
		panic("input should be either *Series or float64")

	}
}

// Divid two series, result[i] = a[i] / b[i]
func Div(a interface{}, b interface{}) Series {
	aa := switchIface(a)
	if 0 == b {
		panic("Divid by zero exception")
	}
	bb := switchIface(b)
	return &DivSeriesResult{aa, bb}

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
func Mul(a interface{}, b interface{}) Series {
	var aa Series
	var bb Series

	switch a.(type) {
	case float64:
		aa = NumberSeries(a.(float64))
	case Series:
		aa = a.(Series)
	default:
		panic("input should be either Series or float64")
	}
	switch b.(type) {
	case float64:
		bb = NumberSeries(b.(float64))
	case Series:
		bb = b.(Series)
	default:
		panic("input should be either Series or float64")

	}
	return &MulSeriesResult{aa, bb}

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
	return Sum(Mul(a, b), limit...)
}

// Extract elements from the Series to a float64 array, following the order of Index(0..limit)
// if limit is given, will only take the first limit numbers (a.Index[0..limit])
// otherwise will operate on all elements
func ToArray(a Series, limit ...int) (result []float64) {
	l := -1
	if len(limit) > 0 {
		l = limit[0]
	}
	if l < a.Length() {
		l = a.Length()
	}
	result = make([]float64, l, l)
	for i := 0; i < l; i++ {
		result[i] = a.Index(i)
	}
	return
}

// Similar to ToArray but in reverse order.
// Useful when you want to cache series' calculated result as float64 array
// the then reuse the result in multiple places (so that no recalculation will be triggered)
//
// notice that the return type is a Float64Slice, which implements the Series interface
func ToReverseArray(a Series, limit ...int) (result Float64Slice) {
	l := -1
	if len(limit) > 0 {
		l = limit[0]
	}
	if l < a.Length() {
		l = a.Length()
	}
	result = make([]float64, l, l)
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
func Change(a Series, offset ...int) Series {
	o := 1
	if len(offset) > 0 {
		o = offset[0]
	}

	return &ChangeResult{a, o}
}

func Stdev(a Series, length int) float64 {
	avg := Mean(a, length)
	s := .0
	for i := 0; i < length; i++ {
		diff := a.Index(i) - avg
		s += diff * diff
	}
	return math.Sqrt(s / float64(length))
}

// TODO: ta.linreg
