package indicator

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

/*
python:

import pandas as pd
import pandas_ta as ta

data = pd.Series([0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9])

high = pd.Series([100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109])

low = pd.Series([80,81,82,83,84,85,86,87,88,89,80,81,82,83,84,85,86,87,88,89,80,81,82,83,84,85,86,87,88,89])

close = pd.Series([90,91,92,93,94,95,96,97,98,99,90,91,92,93,94,95,96,97,98,99,90,91,92,93,94,95,96,97,98,99])

result = ta.adx(high, low, close, 5, 14)
print(result['ADX_14'])

print(result['DMP_5'])
print(result['DMN_5'])
*/
func Test_DMI(t *testing.T) {
	var Delta = 0.001
	var highb = []byte(`[100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109]`)
	var lowb = []byte(`[80,81,82,83,84,85,86,87,88,89,80,81,82,83,84,85,86,87,88,89,80,81,82,83,84,85,86,87,88,89]`)
	var clozeb = []byte(`[90,91,92,93,94,95,96,97,98,99,90,91,92,93,94,95,96,97,98,99,90,91,92,93,94,95,96,97,98,99]`)

	buildKLines := func(h, l, c []byte) (klines []types.KLine) {
		var hv, cv, lv []fixedpoint.Value
		_ = json.Unmarshal(h, &hv)
		_ = json.Unmarshal(l, &lv)
		_ = json.Unmarshal(c, &cv)
		if len(hv) != len(lv) || len(lv) != len(cv) {
			panic(fmt.Sprintf("length not equal %v %v %v", len(hv), len(lv), len(cv)))
		}
		for i, hh := range hv {
			kline := types.KLine{High: hh, Low: lv[i], Close: cv[i]}
			klines = append(klines, kline)
		}
		return klines
	}

	type output struct {
		dip float64
		dim float64
		adx float64
	}

	tests := []struct {
		name   string
		klines []types.KLine
		want   output
		next   output
		total  int
	}{
		{
			name:   "test_dmi",
			klines: buildKLines(highb, lowb, clozeb),
			want:   output{dip: 4.85114, dim: 1.339736, adx: 37.857156},
			next:   output{dip: 4.813853, dim: 1.67532, adx: 36.111434},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dmi := &DMI{
				IntervalWindow: types.IntervalWindow{Window: 5},
				ADXSmoothing:   14,
			}
			dmi.CalculateAndUpdate(tt.klines)
			assert.InDelta(t, dmi.GetDIPlus().Last(0), tt.want.dip, Delta)
			assert.InDelta(t, dmi.GetDIMinus().Last(0), tt.want.dim, Delta)
			assert.InDelta(t, dmi.GetADX().Last(0), tt.want.adx, Delta)
		})
	}

}

// Test_DMI_FlatMarketKeepsFinite verifies that candles with zero volatility do
// not poison the Wilder smoothing state. Exchanges emit flat candles
// (high == low == close) for intervals without trades, which makes ATR and both
// directional movements exactly zero. The ungated divisions then produce Inf /
// NaN, and because RMA keeps its smoothed value in internal state, the ADX
// stays NaN forever even after directional movement resumes.
func Test_DMI_FlatMarketKeepsFinite(t *testing.T) {
	dmi := &DMI{
		IntervalWindow: types.IntervalWindow{Window: 5},
		ADXSmoothing:   14,
	}

	// a long stretch of no-trade candles at a flat price
	for i := 0; i < 20; i++ {
		dmi.Update(100, 100, 100)
	}

	assert.False(t, math.IsNaN(dmi.GetADX().Last(0)), "ADX must stay finite on flat candles")
	assert.False(t, math.IsNaN(dmi.GetDIPlus().Last(0)), "DI+ must stay finite on flat candles")
	assert.False(t, math.IsNaN(dmi.GetDIMinus().Last(0)), "DI- must stay finite on flat candles")

	// zero volatility and zero directional movement means no trend
	assert.Equal(t, 0.0, dmi.GetADX().Last(0))
	assert.Equal(t, 0.0, dmi.GetDIPlus().Last(0))
	assert.Equal(t, 0.0, dmi.GetDIMinus().Last(0))

	// directional movement resumes: the indicator must recover with finite values
	for i := 1; i <= 40; i++ {
		cloze := float64(100 + i)
		dmi.Update(cloze+1, cloze-1, cloze)
	}

	adx := dmi.GetADX().Last(0)
	diPlus := dmi.GetDIPlus().Last(0)
	diMinus := dmi.GetDIMinus().Last(0)
	assert.False(t, math.IsNaN(adx), "ADX must recover after the flat stretch")
	assert.False(t, math.IsInf(diPlus, 0), "DI+ must stay finite after the flat stretch")
	assert.False(t, math.IsInf(diMinus, 0), "DI- must stay finite after the flat stretch")
	assert.Greater(t, adx, 0.0, "a steady uptrend after a flat stretch must produce a positive ADX")
	assert.LessOrEqual(t, adx, 100.0)
}
