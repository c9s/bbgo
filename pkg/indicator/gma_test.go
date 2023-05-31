package indicator

import (
	"encoding/json"
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

/*
python:

import pandas as pd
from scipy.stats.mstats import gmean

data = pd.Series([1.1,1.2,1.3,1.4,1.5,1.6,1.7,1.8,1.9,1.1,1.2,1.3,1.4,1.5,1.6,1.7,1.8,1.9,1.1,1.2,1.3,1.4,1.5,1.6,1.7,1.8,1.9])
gmean(data[-5:])
gmean(data[-6:-1])
gmean(pd.concat(data[-4:], pd.Series([1.3])))
*/
func Test_GMA(t *testing.T) {
	var randomPrices = []byte(`[1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9, 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9, 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9]`)
	var input []fixedpoint.Value
	if err := json.Unmarshal(randomPrices, &input); err != nil {
		panic(err)
	}
	tests := []struct {
		name         string
		kLines       []types.KLine
		want         float64
		next         float64
		update       float64
		updateResult float64
		all          int
	}{
		{
			name:         "test",
			kLines:       buildKLines(input),
			want:         1.6940930229200213,
			next:         1.5937204331251167,
			update:       1.3,
			updateResult: 1.6462950504034335,
			all:          24,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gma := GMA{IntervalWindow: types.IntervalWindow{Window: 5}}
			for _, k := range tt.kLines {
				gma.PushK(k)
			}
			assert.InDelta(t, tt.want, gma.Last(0), Delta)
			assert.InDelta(t, tt.next, gma.Index(1), Delta)
			gma.Update(tt.update)
			assert.InDelta(t, tt.updateResult, gma.Last(0), Delta)
			assert.Equal(t, tt.all, gma.Length())
		})
	}
}
