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

data = pd.Series([0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9])
out = ta.psar(data, data)
print(out)
*/

func Test_PSAR(t *testing.T) {
	var randomPrices = []byte(`[0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9]`)
	var input []fixedpoint.Value
	if err := json.Unmarshal(randomPrices, &input); err != nil {
		panic(err)
	}
	psar := PSAR{
		IntervalWindow: types.IntervalWindow{Window: 2},
	}
	for _, v := range input {
		kline := types.KLine{
			High: v,
			Low:  v,
		}
		psar.PushK(kline)
	}
	assert.Equal(t, psar.Length(), 29)
	assert.Equal(t, psar.AF, 0.04)
	assert.Equal(t, psar.Last(0), 0.16)
}
