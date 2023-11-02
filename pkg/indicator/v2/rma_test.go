package indicatorv2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

/*
python

import pandas as pd
import pandas_ta as ta

data = [40105.78, 39935.23, 40183.97, 40182.03, 40212.26, 40149.99, 40378.0, 40618.37, 40401.03, 39990.39, 40179.13, 40097.23, 40014.72, 39667.85, 39303.1, 39519.99, 39693.79, 39827.96, 40074.94, 40059.84]

close = pd.Series(data)
result = ta.rma(close, length=14)
print(result)
*/
func Test_RMA2(t *testing.T) {
	var bytes = []byte(`[40105.78, 39935.23, 40183.97, 40182.03, 40212.26, 40149.99, 40378.0, 40618.37, 40401.03, 39990.39, 40179.13, 40097.23, 40014.72, 39667.85, 39303.1, 39519.99, 39693.79, 39827.96, 40074.94, 40059.84]`)
	var values []float64
	err := json.Unmarshal(bytes, &values)
	assert.NoError(t, err)

	prices := ClosePrices(nil)
	for _, v := range values {
		prices.Push(v)
	}

	tests := []struct {
		name   string
		window int
		want   []float64
	}{
		{
			name:   "test_binance_btcusdt_1h",
			window: 14,
			want: []float64{
				40129.841000,
				40041.830291,
				39988.157743,
				39958.803719,
				39946.115094,
				39958.296741,
				39967.681562,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rma := RMA2(prices, tt.window, true)
			if assert.Equal(t, len(tt.want), len(rma.Slice)-tt.window+1) {
				for i, v := range tt.want {
					j := tt.window - 1 + i
					got := rma.Slice[j]
					assert.InDelta(t, v, got, 0.01, "Expected rma.slice[%d] to be %v, but got %v", j, v, got)
				}
			}
		})
	}
}
