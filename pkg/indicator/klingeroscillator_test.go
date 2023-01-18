package indicator

import (
	"encoding/json"
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

/*
import pandas as pd
import pandas_ta as ta
high = pd.Series([1.1, 1.3, 1.5, 1.7, 1.9, 2.2, 2.4, 2.1, 1.8, 1.7])
low = pd.Series([0.9, 1.1, 1.2, 1.5, 1.7, 2.0, 2.2, 1.9, 1.6, 1.5])
close = pd.Series([1.0, 1.2, 1.4, 1.6, 1.8, 2.1, 2.3, 2.0, 1.7, 1.6])
vol = pd.Series([300., 200., 200., 150., 150., 200., 200., 150., 300., 350.])
# kvo = ta.kvo(high, low, close, vol, fast=3, slow=5, signal=1)
# print(kvo)
# # The implementation of kvo in pandas_ta is different from the one defined in investopedia
# # VF is not simply multipying trend
# # Also the value is not multiplied by 100 in pandas_ta
*/

func Test_KlingerOscillator(t *testing.T) {
	var high, low, cloze, vResult, vol []fixedpoint.Value
	if err := json.Unmarshal([]byte(`[1.1, 1.3, 1.5, 1.7, 1.9, 2.2, 2.4, 2.1, 1.8, 1.7]`), &high); err != nil {
		panic(err)
	}
	if err := json.Unmarshal([]byte(`[0.9, 1.1, 1.2, 1.5, 1.7, 2.0, 2.2, 1.9, 1.6, 1.5]`), &low); err != nil {
		panic(err)
	}
	if err := json.Unmarshal([]byte(`[1.0, 1.2, 1.4, 1.6, 1.8, 2.1, 2.3, 2.0, 1.7, 1.6]`), &cloze); err != nil {
		panic(err)
	}
	if err := json.Unmarshal([]byte(`[300.0, 200.0, 200.0, 150.0, 150.0, 200.0, 200.0, 150.0, 300.0, 350.0]`), &vol); err != nil {
		panic(err)
	}
	if err := json.Unmarshal([]byte(`[300.0, 0.0, -28.5, -83, -95, -138, -146.7, 0, 100, 175]`), &vResult); err != nil {
		panic(err)
	}

	k := KlingerOscillator{
		Fast: &EWMA{IntervalWindow: types.IntervalWindow{Window: 3}},
		Slow: &EWMA{IntervalWindow: types.IntervalWindow{Window: 5}},
	}
	var Delta = 0.5
	for i := 0; i < len(high); i++ {
		k.Update(high[i].Float64(), low[i].Float64(), cloze[i].Float64(), vol[i].Float64())
		assert.InDelta(t, k.VF.Value, vResult[i].Float64(), Delta)
	}
}
