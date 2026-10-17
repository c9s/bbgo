package indicator

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_ADX(t *testing.T) {
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

	tests := []struct {
		name    string
		klines  []types.KLine
		wantADX float64
	}{
		{
			name:    "test_adx",
			klines:  buildKLines(highb, lowb, clozeb),
			wantADX: 37.857156,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adx := &ADX{
				IntervalWindow: types.IntervalWindow{Window: 5},
				ADXSmoothing:   14,
			}
			adx.CalculateAndUpdate(tt.klines)
			assert.InDelta(t, adx.Last(0), tt.wantADX, Delta)
			assert.Equal(t, adx.Length(), adx.dmi.ADX.Length())
			assert.InDelta(t, adx.Index(0), tt.wantADX, Delta)
		})
	}
}
