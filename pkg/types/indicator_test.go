package types

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
