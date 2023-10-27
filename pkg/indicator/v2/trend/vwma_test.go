package trend

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestVwma(t *testing.T) {

	tests := []struct {
		name    string
		closing []float64
		volume  []float64
		window  int
		want    []float64
	}{
		{
			name:    "Valid sample",
			closing: []float64{20, 21, 21, 19, 16},
			volume:  []float64{100, 50, 40, 50, 100},
			window:  3,
			want:    []float64{20, 20.33, 20.47, 20.29, 17.84},
		},
		{
			name:    "Default window",
			closing: []float64{20, 21, 21, 19, 16},
			volume:  []float64{100, 50, 40, 50, 100},
			window:  20,
			want:    []float64{20, 20.33, 20.47, 20.17, 18.94},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := &types.StandardStream{}
			kLines := v2.KLines(stream, "", "")
			ind := Vwma(kLines, tt.window)
			var ts []types.KLine
			for i := range tt.closing {
				kline := types.KLine{Volume: n(tt.volume[i]), Close: n(tt.closing[i])}
				ts = append(ts, kline)
			}
			for _, candle := range ts {
				stream.EmitKLineClosed(candle)
			}
			spew.Dump(ind)
			for i, v := range tt.want {
				assert.InDelta(t, v, ind.Slice[i], 0.01, "Expected TEMA.slice[%d] to be %v, but got %v", i, v, ind.Slice[i])
			}
		})
	}
}
