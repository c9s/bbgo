package indicator

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

var randomPrices = []float64{0.6046702879796195, 0.9405190880450124, 0.6645700532184904, 0.4377241871869802, 0.4246474970712657, 0.6868330728671094, 0.06564701921747622, 0.15652925473279125, 0.09697951891448456, 0.3009218605852871}
var randomVolumes = []float64{0.5152226285020653, 0.8136499609900968, 0.21427387258237493, 0.380667189299686, 0.31806817433032986, 0.4688998449024232, 0.2830441511804452, 0.2931118573368158, 0.6790946759202162, 0.2185630525927643}

func Test_calculateVWAP(t *testing.T) {
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
		want   float64
	}{
		{
			name:   "trivial_case",
			kLines: buildKLines([]float64{0}, []float64{1}),
			window: 0,
			want:   0.0,
		},
		{
			name:   "easy_case",
			kLines: buildKLines([]float64{1, 2, 3}, []float64{4, 5, 6}),
			window: 0,
			want:   (1*4 + 2*5 + 3*6) / float64(4+5+6),
		},
		{
			name:   "window_case",
			kLines: buildKLines([]float64{1, 2, 3, 4}, []float64{4, 5, 6, 7}),
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
			vwap := VWAP{IntervalWindow: types.IntervalWindow{Window: tt.window}}
			priceF := KLineTypicalPriceMapper
			got := vwap.calculateVWAP(tt.kLines, priceF)
			if got != tt.want {
				t.Errorf("calculateVWAP() = %v, want %v", got, tt.want)
			}
		})
	}
}
