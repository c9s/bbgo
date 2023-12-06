package types

import (
	// "os"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wcharczuk/go-chart/v2"
	"gonum.org/v1/gonum/stat"

	"github.com/c9s/bbgo/pkg/datatype/floats"
)

func TestQueue(t *testing.T) {
	zeroq := NewQueue(0)
	assert.Equal(t, zeroq.Last(0), 0.)
	assert.Equal(t, zeroq.Index(0), 0.)
	zeroq.Update(1.)
	assert.Equal(t, zeroq.Length(), 0)
}

func TestFloat(t *testing.T) {
	var a Series = Sub(3., 2.)
	assert.Equal(t, a.Last(0), 1.)
	assert.Equal(t, a.Last(100), 1.)
}

func TestNextCross(t *testing.T) {
	var a Series = NumberSeries(1.2)

	var b Series = &floats.Slice{100., 80., 60.}
	// index                       2    1    0
	// predicted                                40  20  0
	// offset                                   1   2   3

	index, value, ok := NextCross(a, b, 3)
	assert.True(t, ok)
	assert.Equal(t, value, 1.2)
	assert.Equal(t, index, 3) // 2.94, ceil
}

func TestFloat64Slice(t *testing.T) {
	var a = floats.Slice{1.0, 2.0, 3.0}
	var b = floats.Slice{1.0, 2.0, 3.0}
	var c Series = Sub(&a, &b)
	a = append(a, 4.0)
	b = append(b, 3.0)
	assert.Equal(t, c.Last(0), 1.)
}

/*
python

import pandas as pd
s1 = pd.Series([.2, 0., .6, .2, .2])
s2 = pd.Series([.3, .6, .0, .1])
print(s1.corr(s2, method='pearson'))
print(s1.corr(s2, method='spearman')
print(s1.corr(s2, method='kendall'))
print(s1.rank())
*/
func TestCorr(t *testing.T) {
	var a = floats.Slice{.2, .0, .6, .2}
	var b = floats.Slice{.3, .6, .0, .1}
	corr := Correlation(&a, &b, 4, Pearson)
	assert.InDelta(t, corr, -0.8510644, 0.001)
	out := Rank(&a, 4)
	assert.Equal(t, out.Last(0), 2.5)
	assert.Equal(t, out.Last(1), 4.0)
	corr = Correlation(&a, &b, 4, Spearman)
	assert.InDelta(t, corr, -0.94868, 0.001)
}

/*
python

import pandas as pd
s1 = pd.Series([.2, 0., .6, .2, .2])
s2 = pd.Series([.3, .6, .0, .1])
print(s1.cov(s2, ddof=0))
*/
func TestCov(t *testing.T) {
	var a = floats.Slice{.2, .0, .6, .2}
	var b = floats.Slice{.3, .6, .0, .1}
	cov := Covariance(&a, &b, 4)
	assert.InDelta(t, cov, -0.042499, 0.001)
}

/*
python

import pandas as pd
s1 = pd.Series([.2, 0., .6, .2, .2])
print(s1.skew())
*/
func TestSkew(t *testing.T) {
	var a = floats.Slice{.2, .0, .6, .2}
	sk := Skew(&a, 4)
	assert.InDelta(t, sk, 1.129338, 0.001)
}

func TestEntropy(t *testing.T) {
	var a = floats.Slice{.2, .0, .6, .2}
	e := stat.Entropy(a)
	assert.InDelta(t, e, Entropy(&a, a.Length()), 0.0001)
}

func TestCrossEntropy(t *testing.T) {
	var a = floats.Slice{.2, .0, .6, .2}
	var b = floats.Slice{.3, .6, .0, .1}
	e := stat.CrossEntropy(a, b)
	assert.InDelta(t, e, CrossEntropy(&a, &b, a.Length()), 0.0001)
}

func TestSoftmax(t *testing.T) {
	var a = floats.Slice{3.0, 1.0, 0.2}
	out := Softmax(&a, a.Length())
	r := floats.Slice{0.8360188027814407, 0.11314284146556013, 0.05083835575299916}
	for i := 0; i < out.Length(); i++ {
		assert.InDelta(t, r.Last(i), out.Last(i), 0.001)
	}
}

func TestSigmoid(t *testing.T) {
	a := floats.Slice{3.0, 1.0, 2.1}
	out := Sigmoid(&a)
	r := floats.Slice{0.9525741268224334, 0.7310585786300049, 0.8909031788043871}
	for i := 0; i < out.Length(); i++ {
		assert.InDelta(t, r.Last(i), out.Last(i), 0.001, "i=%d", i)
	}
}

func TestHighLowest(t *testing.T) {
	a := floats.Slice{3.0, 1.0, 2.1}
	assert.Equal(t, 3.0, Highest(&a, 4))
	assert.Equal(t, 1.0, Lowest(&a, 4))
}

func TestAdd(t *testing.T) {
	var a NumberSeries = 3.0
	var b NumberSeries = 2.0
	out := Add(&a, &b)
	assert.Equal(t, out.Last(0), 5.0)
	assert.Equal(t, out.Length(), math.MaxInt32)
}

func TestDiv(t *testing.T) {
	a := floats.Slice{3.0, 1.0, 2.0}
	b := NumberSeries(2.0)
	out := Div(&a, &b)
	assert.Equal(t, 1.0, out.Last(0))
	assert.Equal(t, 3, out.Length())
	assert.Equal(t, 0.5, out.Last(1))
}

func TestMul(t *testing.T) {
	a := floats.Slice{3.0, 1.0, 2.0}
	b := NumberSeries(2.0)
	out := Mul(&a, &b)
	assert.Equal(t, out.Last(0), 4.0)
	assert.Equal(t, out.Length(), 3)
	assert.Equal(t, out.Last(1), 2.0)
}

func TestArray(t *testing.T) {
	a := floats.Slice{3.0, 1.0, 2.0}
	out := Array(&a, 1)
	assert.Equal(t, len(out), 1)
	out = Array(&a, 4)
	assert.Equal(t, len(out), 3)
}

func TestSwitchInterface(t *testing.T) {
	var a int = 1
	var af float64 = 1.0
	var b int32 = 2
	var bf float64 = 2.0
	var c int64 = 3
	var cf float64 = 3.0
	var d float32 = 4.0
	var df float64 = 4.0
	var e float64 = 5.0
	assert.Equal(t, switchIface(a).Last(0), af)
	assert.Equal(t, switchIface(b).Last(0), bf)
	assert.Equal(t, switchIface(c).Last(0), cf)
	assert.Equal(t, switchIface(d).Last(0), df)
	assert.Equal(t, switchIface(e).Last(0), e)
}

// from https://en.wikipedia.org/wiki/Logistic_regression
func TestLogisticRegression(t *testing.T) {
	a := []floats.Slice{{0.5, 0.75, 1., 1.25, 1.5, 1.75, 1.75, 2.0, 2.25, 2.5, 2.75, 3., 3.25, 3.5, 4., 4.25, 4.5, 4.75, 5., 5.5}}
	b := floats.Slice{0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1}
	var x []Series
	x = append(x, &a[0])

	model := LogisticRegression(x, &b, a[0].Length(), 90000, 0.0018)
	inputs := []float64{1., 2., 2.7, 3., 4., 5.}
	results := []bool{false, false, true, true, true, true}
	for i, x := range inputs {
		input := []float64{x}
		pred := model.Predict(input)
		assert.Equal(t, pred >= 0.5, results[i])
	}
}

func TestDot(t *testing.T) {
	a := floats.Slice{7, 6, 5, 4, 3, 2, 1, 0}
	b := floats.Slice{200., 201., 203., 204., 203., 199.}
	out1 := Dot(&a, &b, 3)
	assert.InDelta(t, out1, 611., 0.001)
	out2 := Dot(&a, 3., 2)
	assert.InDelta(t, out2, 3., 0.001)
	out3 := Dot(3., &a, 2)
	assert.InDelta(t, out2, out3, 0.001)
	out4 := Dot(&a, 3, 2)
	assert.InDelta(t, out2, 3., 0.001)
	out5 := Dot(3, &a, 2)
	assert.InDelta(t, out4, out5, 0.001)
}

func TestClone(t *testing.T) {
	a := NewQueue(3)
	a.Update(3.)
	b := Clone(a)
	b.Update(4.)
	assert.Equal(t, a.Last(0), 3.)
	assert.Equal(t, b.Last(0), 4.)
}

func TestPlot(t *testing.T) {
	ct := NewCanvas("test", Interval5m)
	a := floats.Slice{200., 205., 230., 236}
	ct.Plot("test", &a, Time(time.Now()), 4)
	assert.Equal(t, ct.Interval, Interval5m)
	assert.Equal(t, ct.Series[0].(chart.TimeSeries).Len(), 4)
	// f, _ := os.Create("output.png")
	// defer f.Close()
	// ct.Render(chart.PNG, f)
}

func TestFilter(t *testing.T) {
	a := floats.Slice{200., -200, 0, 1000, -100}
	b := Filter(&a, func(i int, val float64) bool {
		return val > 0
	}, 4)
	assert.Equal(t, b.Length(), 4)
	assert.Equal(t, b.Last(0), 1000.)
	assert.Equal(t, b.Sum(3), 1200.)
}
