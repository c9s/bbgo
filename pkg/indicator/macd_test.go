package indicator

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

/*
python:

import pandas as pd
s = pd.Series([0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9])
slow = s.ewm(span=26, adjust=False).mean()
fast = s.ewm(span=12, adjust=False).mean()
print(fast - slow)
*/

func Test_calculateMACD(t *testing.T) {
	var randomPrices = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	tests := []struct {
		name   string
		kLines []types.KLine
		want   float64
	}{
		{
			name:   "random_case",
			kLines: buildKLines(randomPrices),
			want:   0.7967670223776384,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iw := types.IntervalWindow{Window: 9}
			macd := MACD{IntervalWindow: iw, ShortPeriod: 12, LongPeriod: 26}
			priceF := KLineClosePriceMapper
			got := macd.calculateMACD(tt.kLines, priceF)
			if got != tt.want {
				t.Errorf("calculateMACD() = %v, want %v", got, tt.want)
			}
		})
	}
}
