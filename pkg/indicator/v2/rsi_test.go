package indicatorv2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/datatype/floats"
)

func Test_RSI2(t *testing.T) {
	// test case from https://school.stockcharts.com/doku.php?id=technical_indicators:relative_strength_index_rsi
	var data = []byte(`[44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.10, 45.42, 45.84, 46.08, 45.89, 46.03, 45.61, 46.28, 46.28, 46.00, 46.03, 46.41, 46.22, 45.64, 46.21, 46.25, 45.71, 46.45, 45.78, 45.35, 44.03, 44.18, 44.22, 44.57, 43.42, 42.66, 43.13]`)
	var values []float64
	err := json.Unmarshal(data, &values)
	assert.NoError(t, err)

	tests := []struct {
		name   string
		values []float64
		window int
		want   floats.Slice
	}{
		{
			name:   "RSI",
			values: values,
			window: 14,
			// Wilder's RSI, the first value is calculated from the first 14 price changes
			want: floats.Slice{
				70.464135,
				66.249619,
				66.480942,
				69.346853,
				66.294713,
				57.915021,
				62.880718,
				63.208789,
				56.011585,
				62.339929,
				54.670971,
				50.386815,
				40.019424,
				41.492635,
				41.902430,
				45.499497,
				37.322778,
				33.090483,
				37.788772,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// RSI2()
			prices := ClosePrices(nil)
			rsi := RSI2(prices, tt.window)

			t.Logf("data length: %d", len(tt.values))
			for _, price := range tt.values {
				prices.PushAndEmit(price)
			}

			assert.Equal(t, floats.Slice(tt.values), prices.Slice)

			if assert.Equal(t, len(tt.want), len(rsi.Slice)) {
				for i, v := range tt.want {
					assert.InDelta(t, v, rsi.Slice[i], 0.000001, "Expected rsi.slice[%d] to be %v, but got %v", i, v, rsi.Slice[i])
				}
			}
		})
	}
}

func Test_RSI2_OneSided(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{name: "gains only", values: []float64{1, 2, 3, 4, 5}, want: 100},
		{name: "losses only", values: []float64{5, 4, 3, 2, 1}, want: 0},
		{name: "flat", values: []float64{3, 3, 3, 3, 3}, want: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prices := ClosePrices(nil)
			rsi := RSI2(prices, 3)
			for _, v := range tt.values {
				prices.PushAndEmit(v)
			}

			// 4 changes from 5 prices, the first value needs 3 changes
			assert.Equal(t, 2, rsi.Length())
			assert.Equal(t, tt.want, rsi.Last(0))
		})
	}
}
