package types

import (
	"math"

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
	x := make([]float64, 0, lookback)
	y1 := make([]float64, 0, lookback)
	y2 := make([]float64, 0, lookback)
	var weights []float64
	for i := 0; i < lookback; i++ {
		x = append(x, float64(i))
		y1 = append(y1, a.Index(i))
		y2 = append(y2, b.Index(i))
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
		return c.a.Index(i)-c.b.Index(i) > 0 && c.a.Index(i-1)-c.b.Index(i-1) < 0
	} else {
		return c.a.Index(i)-c.b.Index(i) < 0 && c.a.Index(i-1)-c.b.Index(i-1) > 0
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

type Float64ArrSeries []float64

func (a Float64ArrSeries) Last() float64 {
	if len(a) > 0 {
		return a[len(a)-1]
	}
	return 0.0
}

func (a Float64ArrSeries) Index(i int) float64 {
	if len(a)-i < 0 || i < 0 {
		return 0.0
	}
	return a[len(a)-i-1]
}

func (a Float64ArrSeries) Length() int {
	return len(a)
}

var _ Series = Float64ArrSeries([]float64{})

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
	}
	switch b.(type) {
	case float64:
		bb = NumberSeries(b.(float64))
	case Series:
		bb = b.(Series)
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
	var aa Series
	var bb Series

	switch a.(type) {
	case float64:
		aa = NumberSeries(a.(float64))
	case Series:
		aa = a.(Series)
	}
	switch b.(type) {
	case float64:
		bb = NumberSeries(b.(float64))
	case Series:
		bb = b.(Series)
	}
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

// Divid two series, result[i] = a[i] / b[i]
func Div(a interface{}, b interface{}) Series {
	var aa Series
	var bb Series

	switch a.(type) {
	case float64:
		aa = NumberSeries(a.(float64))
	case Series:
		aa = a.(Series)
	}
	switch b.(type) {
	case float64:
		bb = NumberSeries(b.(float64))
		if 0 == bb.Last() {
			panic("Divid by zero exception")
		}
	case Series:
		bb = b.(Series)
	}
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
	}
	switch b.(type) {
	case float64:
		bb = NumberSeries(b.(float64))
	case Series:
		bb = b.(Series)
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
