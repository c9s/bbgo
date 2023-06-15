package indicator

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

/*
python

import pandas as pd
import pandas_ta as ta

data = {
    "high": [40145.0, 40186.36, 40196.39, 40344.6, 40245.48, 40273.24, 40464.0, 40699.0, 40627.48, 40436.31, 40370.0, 40376.8, 40227.03, 40056.52, 39721.7, 39597.94, 39750.15, 39927.0, 40289.02, 40189.0],
    "low": [39870.71, 39834.98, 39866.31, 40108.31, 40016.09, 40094.66, 40105.0, 40196.48, 40154.99, 39800.0, 39959.21, 39922.98, 39940.02, 39632.0, 39261.39, 39254.63, 39473.91, 39555.51, 39819.0, 40006.84],
    "close": [40105.78, 39935.23, 40183.97, 40182.03, 40212.26, 40149.99, 40378.0, 40618.37, 40401.03, 39990.39, 40179.13, 40097.23, 40014.72, 39667.85, 39303.1, 39519.99,
39693.79, 39827.96, 40074.94, 40059.84]
}

high = pd.Series(data['high'])
low = pd.Series(data['low'])
close = pd.Series(data['close'])
result = ta.atr(high, low, close, length=14)
print(result)
*/
func Test_calculateATR(t *testing.T) {
	var bytes = []byte(`{
		"high": [40145.0, 40186.36, 40196.39, 40344.6, 40245.48, 40273.24, 40464.0, 40699.0, 40627.48, 40436.31, 40370.0, 40376.8, 40227.03, 40056.52, 39721.7, 39597.94, 39750.15, 39927.0, 40289.02, 40189.0], 
		"low": [39870.71, 39834.98, 39866.31, 40108.31, 40016.09, 40094.66, 40105.0, 40196.48, 40154.99, 39800.0, 39959.21, 39922.98, 39940.02, 39632.0, 39261.39, 39254.63, 39473.91, 39555.51, 39819.0, 40006.84],
		"close": [40105.78, 39935.23, 40183.97, 40182.03, 40212.26, 40149.99, 40378.0, 40618.37, 40401.03, 39990.39, 40179.13, 40097.23, 40014.72, 39667.85, 39303.1, 39519.99, 39693.79, 39827.96, 40074.94, 40059.84]
	}`)
	buildKLines := func(bytes []byte) (kLines []types.KLine) {
		var prices map[string][]fixedpoint.Value
		_ = json.Unmarshal(bytes, &prices)
		for i, h := range prices["high"] {
			kLine := types.KLine{High: h, Low: prices["low"][i], Close: prices["close"][i]}
			kLines = append(kLines, kLine)
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
			name:   "test_binance_btcusdt_1h",
			kLines: buildKLines(bytes),
			window: 14,
			want:   367.913903,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atr := &ATR{IntervalWindow: types.IntervalWindow{Window: tt.window}}
			for _, k := range tt.kLines {
				atr.PushK(k)
			}

			got := atr.Last(0)
			diff := math.Trunc((got-tt.want)*100) / 100
			if diff != 0 {
				t.Errorf("calculateATR() = %v, want %v", got, tt.want)
			}
		})
	}
}
