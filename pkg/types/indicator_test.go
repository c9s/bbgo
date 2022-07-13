package types

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"gonum.org/v1/gonum/stat"
)

func TestFloat(t *testing.T) {
	var a Series = Minus(3., 2.)
	assert.Equal(t, a.Last(), 1.)
	assert.Equal(t, a.Index(100), 1.)
}

func TestNextCross(t *testing.T) {
	var a Series = NumberSeries(1.2)

	var b Series = &Float64Slice{100., 80., 60.}
	// index                       2    1    0
	// predicted                                40  20  0
	// offset                                   1   2   3

	index, value, ok := NextCross(a, b, 3)
	assert.True(t, ok)
	assert.Equal(t, value, 1.2)
	assert.Equal(t, index, 3) // 2.94, ceil
}

func TestFloat64Slice(t *testing.T) {
	var a = Float64Slice{1.0, 2.0, 3.0}
	var b = Float64Slice{1.0, 2.0, 3.0}
	var c Series = Minus(&a, &b)
	a = append(a, 4.0)
	b = append(b, 3.0)
	assert.Equal(t, c.Last(), 1.)
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
	var a = Float64Slice{.2, .0, .6, .2}
	var b = Float64Slice{.3, .6, .0, .1}
	corr := Correlation(&a, &b, 4, Pearson)
	assert.InDelta(t, corr, -0.8510644, 0.001)
	out := Rank(&a, 4)
	assert.Equal(t, out.Index(0), 2.5)
	assert.Equal(t, out.Index(1), 4.0)
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
	var a = Float64Slice{.2, .0, .6, .2}
	var b = Float64Slice{.3, .6, .0, .1}
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
	var a = Float64Slice{.2, .0, .6, .2}
	sk := Skew(&a, 4)
	assert.InDelta(t, sk, 1.129338, 0.001)
}

func TestEntropy(t *testing.T) {
	var a = Float64Slice{.2, .0, .6, .2}
	e := stat.Entropy(a)
	assert.InDelta(t, e, Entropy(&a, a.Length()), 0.0001)
}

func TestCrossEntropy(t *testing.T) {
	var a = Float64Slice{.2, .0, .6, .2}
	var b = Float64Slice{.3, .6, .0, .1}
	e := stat.CrossEntropy(a, b)
	assert.InDelta(t, e, CrossEntropy(&a, &b, a.Length()), 0.0001)
}

func TestSoftmax(t *testing.T) {
	var a = Float64Slice{3.0, 1.0, 0.2}
	out := Softmax(&a, a.Length())
	r := Float64Slice{0.8360188027814407, 0.11314284146556013, 0.05083835575299916}
	for i := 0; i < out.Length(); i++ {
		assert.InDelta(t, r.Index(i), out.Index(i), 0.001)
	}
}

func TestSigmoid(t *testing.T) {
	a := Float64Slice{3.0, 1.0, 2.1}
	out := Sigmoid(&a)
	r := Float64Slice{0.9525741268224334, 0.7310585786300049, 0.8909031788043871}
	for i := 0; i < out.Length(); i++ {
		assert.InDelta(t, r.Index(i), out.Index(i), 0.001)
	}
}

// from https://en.wikipedia.org/wiki/Logistic_regression
func TestLogisticRegression(t *testing.T) {
	a := []Float64Slice{{0.5, 0.75, 1., 1.25, 1.5, 1.75, 2.0, 2.25, 2.5, 2.75, 3., 3.25, 3.5, 4., 4.25, 4.5, 4.75, 5., 5.5}}
	b := Float64Slice{0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1}
	var x []Series
	x = append(x, &a[0])

	model := LogisticRegression(x, &b, a[0].Length(), 8000, 0.0009)
	inputs := []float64{1., 2., 2.7, 3., 4., 5.}
	results := []bool{false, false, true, true, true, true}
	for i, x := range inputs {
		input := []float64{x}
		pred := model.Predict(input)
		assert.Equal(t, pred > 0.5, results[i])
	}
}

func TestDot(t *testing.T) {
	a := Float64Slice{7, 6, 5, 4, 3, 2, 1, 0}
	b := Float64Slice{200., 201., 203., 204., 203., 199.}
	out1 := Dot(&a, &b, 3)
	assert.InDelta(t, out1, 611., 0.001)
	out2 := Dot(&a, 3., 2)
	assert.InDelta(t, out2, 3., 0.001)
	out3 := Dot(3., &a, 2)
	assert.InDelta(t, out2, out3, 0.001)
}

func TestSum(t *testing.T) {
	type args struct {
		a     Series
		limit []int
	}
	tests := []struct {
		name    string
		args    args
		wantSum float64
	}{
		{name: "use limit",
			args: args{
				a:     &Float64Slice{0.5, 0.75, 1., 1.25, 1.5, 1.75},
				limit: []int{3},
			},
			wantSum: 4.5,
		},
		{name: "no use limit",
			args: args{
				a:     &Float64Slice{0.5, 0.75, 1., 1.25, 1.5, 1.75},
				limit: nil,
			},
			wantSum: 6.75,
		},
		{name: "over limit",
			args: args{
				a:     &Float64Slice{0.5, 0.75, 1., 1.25, 1.5, 1.75},
				limit: []int{7},
			},
			wantSum: 6.75,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotSum := Sum(tt.args.a, tt.args.limit...); gotSum != tt.wantSum {
				t.Errorf("Sum() = %v, want %v", gotSum, tt.wantSum)
			}
		})
	}
}

func TestMean(t *testing.T) {
	type args struct {
		a     Series
		limit []int
	}
	tests := []struct {
		name     string
		args     args
		wantMean float64
	}{
		{name: "use limit",
			args: args{
				a:     &Float64Slice{0.5, 0.75, 1., 1.25, 1.5, 1.75},
				limit: []int{3},
			},
			wantMean: 1.5,
		},
		{name: "no use limit",
			args: args{
				a:     &Float64Slice{0.5, 0.75, 1., 1.25, 1.5, 1.75},
				limit: nil,
			},
			wantMean: 1.125,
		},
		{name: "over limit",
			args: args{
				a:     &Float64Slice{0.5, 0.75, 1., 1.25, 1.5, 1.75},
				limit: []int{7},
			},
			wantMean: 1.125,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotMean := Mean(tt.args.a, tt.args.limit...); gotMean != tt.wantMean {
				t.Errorf("Mean() = %v, want %v", gotMean, tt.wantMean)
			}
		})
	}
}

func TestArray(t *testing.T) {
	type args struct {
		a     Series
		limit []int
	}
	tests := []struct {
		name       string
		args       args
		wantResult []float64
	}{
		{name: "use limit",
			args: args{
				a:     &Float64Slice{0.5, 0.75, 1., 1.25, 1.5, 1.75},
				limit: []int{3},
			},
			wantResult: []float64{1.75, 1.5, 1.25},
		},
		{name: "no use limit",
			args: args{
				a:     &Float64Slice{0.5, 0.75, 1., 1.25, 1.5, 1.75},
				limit: nil,
			},
			wantResult: []float64{1.75, 1.5, 1.25, 1., 0.75, 0.5},
		},
		{name: "over limit",
			args: args{
				a:     &Float64Slice{0.5, 0.75, 1., 1.25, 1.5, 1.75},
				limit: []int{7},
			},
			wantResult: []float64{1.75, 1.5, 1.25, 1., 0.75, 0.5},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotResult := Array(tt.args.a, tt.args.limit...); !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("Array() = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}

func TestReverse(t *testing.T) {
	type args struct {
		a     Series
		limit []int
	}
	tests := []struct {
		name       string
		args       args
		wantResult Float64Slice
	}{
		{name: "use limit",
			args: args{
				a:     &Float64Slice{0.5, 0.75, 1., 1.25, 1.5, 1.75},
				limit: []int{3},
			},
			wantResult: []float64{1.25, 1.5, 1.75},
		},
		{name: "no use limit",
			args: args{
				a:     &Float64Slice{0.5, 0.75, 1., 1.25, 1.5, 1.75},
				limit: nil,
			},
			wantResult: []float64{0.5, 0.75, 1., 1.25, 1.5, 1.75},
		},
		{name: "over limit",
			args: args{
				a:     &Float64Slice{0.5, 0.75, 1., 1.25, 1.5, 1.75},
				limit: []int{7},
			},
			wantResult: []float64{0.5, 0.75, 1., 1.25, 1.5, 1.75},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotResult := Reverse(tt.args.a, tt.args.limit...); !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("Reverse() = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}
