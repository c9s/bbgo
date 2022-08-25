package indicator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const Delta = 1e-9

func Test_calculateOBV(t *testing.T) {
	buildKLines := func(prices, volumes []fixedpoint.Value) (kLines []types.KLine) {
		for i, p := range prices {
			kLines = append(kLines, types.KLine{High: p, Low: p, Close: p, Volume: volumes[i]})
		}
		return kLines
	}
	var easy1 = []byte(`[3, 2, 1, 4]`)
	var easy2 = []byte(`[3, 2, 2, 6]`)
	var input1 []fixedpoint.Value
	var input2 []fixedpoint.Value
	_ = json.Unmarshal(easy1, &input1)
	_ = json.Unmarshal(easy2, &input2)

	tests := []struct {
		name   string
		kLines []types.KLine
		window int
		want   floats.Slice
	}{
		{
			name: "trivial_case",
			kLines: buildKLines(
				[]fixedpoint.Value{fixedpoint.Zero}, []fixedpoint.Value{fixedpoint.One},
			),
			window: 0,
			want:   floats.Slice{1.0},
		},
		{
			name:   "easy_case",
			kLines: buildKLines(input1, input2),
			window: 0,
			want:   floats.Slice{3, 1, -1, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obv := OBV{IntervalWindow: types.IntervalWindow{Window: tt.window}}
			obv.CalculateAndUpdate(tt.kLines)
			assert.Equal(t, len(obv.Values), len(tt.want))
			for i, v := range obv.Values {
				assert.InDelta(t, v, tt.want[i], Delta)
			}
		})
	}
}
