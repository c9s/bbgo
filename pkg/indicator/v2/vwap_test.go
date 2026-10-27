package indicatorv2

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_VWAP(t *testing.T) {
	var trivialPrices = []byte(`[0]`)
	var trivialVolumes = []byte(`[1]`)
	var easyPrices = []byte(`[1, 2, 3]`)
	var easyVolumes = []byte(`[4, 5, 6]`)
	var windowPrices = []byte(`[1, 2, 3, 4]`)
	var windowVolumes = []byte(`[4, 5, 6, 7]`)
	var randomPrices = []byte(`[0.6046702879796195, 0.9405190880450124, 0.6645700532184904, 0.4377241871869802, 0.4246474970712657, 0.6868330728671094, 0.06564701921747622, 0.15652925473279125, 0.09697951891448456, 0.3009218605852871]`)
	var randomVolumes = []byte(`[0.5152226285020653, 0.8136499609900968, 0.21427387258237493, 0.380667189299686, 0.31806817433032986, 0.4688998449024232, 0.2830441511804452, 0.2931118573368158, 0.6790946759202162, 0.2185630525927643]`)

	buildKLines := func(pb, vb []byte) (kLines []types.KLine) {
		var prices, volumes []fixedpoint.Value
		_ = json.Unmarshal(pb, &prices)
		_ = json.Unmarshal(vb, &volumes)
		for i, p := range prices {
			kLines = append(kLines, types.KLine{High: p, Low: p, Close: p, Volume: volumes[i]})
		}
		return kLines
	}

	tests := []struct {
		name   string
		kLines []types.KLine
		window int
		want   float64
	}{
		{
			name:   "trivial_case",
			kLines: buildKLines(trivialPrices, trivialVolumes),
			window: 0,
			want:   0.0,
		},
		{
			name:   "easy_case",
			kLines: buildKLines(easyPrices, easyVolumes),
			window: 0,
			want:   (1*4 + 2*5 + 3*6) / float64(4+5+6),
		},
		{
			name:   "window_case",
			kLines: buildKLines(windowPrices, windowVolumes),
			window: 3,
			want:   (2*5 + 3*6 + 4*7) / float64(5+6+7),
		},
		{
			name:   "random_case",
			kLines: buildKLines(randomPrices, randomVolumes),
			window: 0,
			want:   0.48727133857423566,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := &KLineStream{}
			vwap := VWAP(stream, tt.window)
			stream.BackFill(tt.kLines)

			assert.Equal(t, len(tt.kLines), vwap.Length())
			// the inputs go through fixedpoint, so allow a small rounding error
			assert.InDelta(t, tt.want, vwap.Last(0), 1e-6)
		})
	}

	for _, window := range []int{0, 5, 20} {
		t.Run(fmt.Sprintf("compare_with_legacy_window_%d", window), func(t *testing.T) {
			kLines := randomWalkKLines(200, 7)

			stream := &KLineStream{}
			vwap := VWAP(stream, window)
			stream.BackFill(kLines)

			legacy := &indicator.VWAP{IntervalWindow: types.IntervalWindow{Window: window}}
			for _, k := range kLines {
				legacy.PushK(k)
			}

			assert.Equal(t, legacy.Length(), vwap.Length())
			assert.InDeltaSlice(t, []float64(legacy.Values), []float64(vwap.Slice), 1e-9)
		})
	}
}
