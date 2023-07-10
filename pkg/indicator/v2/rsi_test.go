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
			want: floats.Slice{
				100.000000,
				99.439336,
				99.440090,
				98.251826,
				98.279242,
				98.297781,
				98.307626,
				98.319149,
				98.334036,
				98.342426,
				97.951933,
				97.957908,
				97.108036,
				97.147514,
				70.464135,
				70.020964,
				69.831224,
				80.567686,
				73.333333,
				59.806295,
				62.528217,
				60.000000,
				48.477752,
				53.878407,
				48.952381,
				43.862816,
				37.732919,
				32.263514,
				32.718121,
				38.142620,
				31.748252,
				25.099602,
				30.217670,
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
