package indicator

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_calculateRSI(t *testing.T) {
	// test case from https://school.stockcharts.com/doku.php?id=technical_indicators:relative_strength_index_rsi
	buildKLines := func(prices []fixedpoint.Value) (kLines []types.KLine) {
		for _, p := range prices {
			kLines = append(kLines, types.KLine{High: p, Low: p, Close: p})
		}
		return kLines
	}
	var data = []byte(`[44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.10, 45.42, 45.84, 46.08, 45.89, 46.03, 45.61, 46.28, 46.28, 46.00, 46.03, 46.41, 46.22, 45.64, 46.21, 46.25, 45.71, 46.45, 45.78, 45.35, 44.03, 44.18, 44.22, 44.57, 43.42, 42.66, 43.13]`)
	var values []fixedpoint.Value
	_ = json.Unmarshal(data, &values)

	tests := []struct {
		name   string
		kLines []types.KLine
		window int
		want   types.Float64Slice
	}{
		{
			name:   "RSI",
			kLines: buildKLines(values),
			window: 14,
			want:   types.Float64Slice{70.53, 66.32, 66.55, 69.41, 66.36, 57.97, 62.93, 63.26, 56.06, 62.38, 54.71, 50.42, 39.99, 41.46, 41.87, 45.46, 37.30, 33.08, 37.77},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rsi := RSI{IntervalWindow: types.IntervalWindow{Window: tt.window}}
			rsi.calculateAndUpdate(tt.kLines)
			fmt.Println(len(tt.want), len(rsi.Values))
			assert.Equal(t, len(rsi.Values), len(tt.want))
			for i, v := range rsi.Values {
				fmt.Println(v, tt.want[i])
				assert.InDelta(t, v, tt.want[i], 0.1)
			}
		})
	}
}
