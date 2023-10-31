package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_AwesomeOscillator(t *testing.T) {
	tests := []struct {
		name string
		high []float64
		low  []float64
		want floats.Slice
	}{
		{
			name: "AwesomeOscillator",
			high: []float64{24.63, 24.69, 24.99, 25.36, 25.19, 25.17, 25.01, 24.96, 25.08, 25.25, 25.21, 25.37, 25.61, 25.58, 25.46, 25.33, 25.09, 25.03, 24.91, 24.89, 25.13, 24.63, 24.69, 24.99, 25.36, 25.19, 25.17, 25.01, 24.96, 25.08, 25.25, 25.21, 25.37, 25.61, 25.58, 25.46, 25.33, 25.09, 25.03, 24.91, 24.89, 25.13},
			low:  []float64{24.63, 24.69, 24.99, 25.36, 25.19, 25.17, 25.01, 24.96, 25.08, 25.25, 25.21, 25.37, 25.61, 25.58, 25.46, 25.33, 25.09, 25.03, 24.91, 24.89, 25.13, 24.63, 24.69, 24.99, 25.36, 25.19, 25.17, 25.01, 24.96, 25.08, 25.25, 25.21, 25.37, 25.61, 25.58, 25.46, 25.33, 25.09, 25.03, 24.91, 24.89, 25.13},
			want: floats.Slice{0.0, 0.17, 0.24, 0.26, 0.28, 0.23, 0.12, -0.01, -0.12, -0.16},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := &types.StandardStream{}
			prices := HL2(KLines(stream, "", ""))

			ind := AwesomeOscillator(prices)

			for _, k := range buildKLinesFromHL(tt.high, tt.low) {
				stream.EmitKLineClosed(k)

			}

			if assert.Equal(t, len(tt.want)+32, len(ind.Slice)) {
				for i, v := range tt.want {
					assert.InDelta(t, v, ind.Slice[i+32], 0.005, "Expected awesome_osc.slice[%d] to be %v, but got %v", i, v, ind.Slice[i])
				}
			}
		})
	}
}

func buildKLinesFromHL(high, low []float64) (klines []types.KLine) {
	for i := range high {
		klines = append(klines, types.KLine{High: n(high[i]), Low: n(low[i])})
	}

	return klines
}

func buildKLinesFromC(closing []float64) (klines []types.KLine) {
	for i := range closing {
		klines = append(klines, types.KLine{Close: n(closing[i])})
	}

	return klines
}
