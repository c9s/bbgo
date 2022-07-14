package indicator

import (
	"encoding/json"
	"fmt"
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
			assert.InDelta(t, dmi.GetDIPlus().Last(), tt.want.dip, Delta)
			assert.InDelta(t, dmi.GetDIMinus().Last(), tt.want.dim, Delta)
			assert.InDelta(t, dmi.GetADX().Last(), tt.want.adx, Delta)
		})
	}

}
