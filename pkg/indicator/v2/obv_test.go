package indicatorv2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_OBV(t *testing.T) {
	buildKLines := func(prices, volumes []fixedpoint.Value) (kLines []types.KLine) {
		for i, p := range prices {
			kLines = append(kLines, types.KLine{High: p, Low: p, Close: p, Volume: volumes[i]})
		}
		return kLines
	}

	parse := func(b string) (out []fixedpoint.Value) {
		_ = json.Unmarshal([]byte(b), &out)
		return out
	}

	tests := []struct {
		name   string
		kLines []types.KLine
		want   []float64
	}{
		{
			name:   "trivial_case",
			kLines: buildKLines([]fixedpoint.Value{fixedpoint.Zero}, []fixedpoint.Value{fixedpoint.One}),
			want:   []float64{1.0},
		},
		{
			// ported from the legacy test
			name:   "easy_case",
			kLines: buildKLines(parse(`[3, 2, 1, 4]`), parse(`[3, 2, 2, 6]`)),
			want:   []float64{3, 1, -1, 5},
		},
		{
			// rising, unchanged, falling and rising again
			name:   "unchanged_close",
			kLines: buildKLines(parse(`[10, 11, 11, 9, 12]`), parse(`[5, 7, 100, 3, 4]`)),
			want:   []float64{5, 12, 12, 9, 13},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := &KLineStream{}
			obv := OBV(stream)
			stream.BackFill(tt.kLines)

			assert.Equal(t, len(tt.want), obv.Length())
			assert.InDeltaSlice(t, tt.want, []float64(obv.Slice), 1e-9)
		})
	}
}
