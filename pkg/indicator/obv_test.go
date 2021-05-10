package indicator

import (
	"reflect"
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_calculateAndUpdate(t *testing.T) {
	buildKLines := func(prices, volumes []float64) (kLines []types.KLine) {
		for i, p := range prices {
			kLines = append(kLines, types.KLine{High: p, Low: p, Close: p, Volume: volumes[i]})
		}
		return kLines
	}

	tests := []struct {
		name   string
		kLines []types.KLine
		window int
		want   Float64Slice
	}{
		{
			name:   "trivial_case",
			kLines: buildKLines([]float64{0}, []float64{1}),
			window: 0,
			want:   Float64Slice{1.0},
		},
		{
			name:   "easy_case",
			kLines: buildKLines([]float64{3, 2, 1, 4}, []float64{3, 2, 2, 6}),
			window: 0,
			want:   Float64Slice{3, 1, -1, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obv := OBV{IntervalWindow: types.IntervalWindow{Window: tt.window}}
			obv.calculateAndUpdate(tt.kLines)
			if !reflect.DeepEqual(obv.Values, tt.want) {
				t.Errorf("calculateAndUpdate() = %v, want %v", obv.Values, tt.want)
			}
		})
	}
}
