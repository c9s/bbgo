package indicator

import (
	"encoding/json"
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/stretchr/testify/assert"
)

/*
import pandas as pd

data = pd.Series([0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9])
ma1 = data.diff(1).ewm(span=25, adjust=False).mean()
ma2 = ma1.ewm(span=13, adjust=False).mean()
ma3 = data.diff(1).abs().ewm(span=25, adjust=False).mean()
ma4 = ma3.ewm(span=13, adjust=False).mean()
print(ma2/ma4*100.)
*/

func Test_TSI(t *testing.T) {
	var randomPrices = []byte(`[0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9]`)
	var input []fixedpoint.Value
	if err := json.Unmarshal(randomPrices, &input); err != nil {
		panic(err)
	}
	tsi := TSI{}
	klines := buildKLines(input)
	for _, kline := range klines {
		tsi.PushK(kline)
	}
	assert.Equal(t, tsi.Length(), 29)
	Delta := 1.5e-2
	assert.InDelta(t, tsi.Last(0), 22.89, Delta)
}
