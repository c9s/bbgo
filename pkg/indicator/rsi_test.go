package indicator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/datatype/floats"
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
		want   floats.Slice
	}{
		{
			name:   "RSI",
			kLines: buildKLines(values),
			window: 14,
			want: floats.Slice{
				70.46413502109704,
				66.24961855355505,
				66.48094183471265,
				69.34685316290864,
				66.29471265892624,
				57.91502067008556,
				62.88071830996241,
				63.208788718287764,
				56.01158478954758,
				62.33992931089789,
				54.67097137765515,
				50.386815195114224,
				40.01942379131357,
				41.49263540422282,
				41.902429678458105,
				45.499497238680405,
				37.32277831337995,
				33.090482572723396,
				37.78877198205783,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rsi := RSI{IntervalWindow: types.IntervalWindow{Window: tt.window}}
			rsi.CalculateAndUpdate(tt.kLines)
			assert.Equal(t, len(rsi.Values), len(tt.want))
			for i, v := range rsi.Values {
				assert.InDelta(t, v, tt.want[i], Delta)
			}
		})
	}
}
