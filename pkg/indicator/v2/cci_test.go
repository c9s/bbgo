package indicatorv2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

/*
python:

import pandas as pd
s = pd.Series([0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9])
ma = s.rolling(16).mean()
md = s.rolling(16).apply(lambda x: (x - x.mean()).abs().mean())
cci = pd.Series((s - ma) / (0.015 * md), name="CCI")
print(cci)
*/
func Test_CCI(t *testing.T) {
	var randomPrices = []byte(`[0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9]`)
	var input []float64
	var delta = 1e-6
	if err := json.Unmarshal(randomPrices, &input); err != nil {
		panic(err)
	}
	t.Run("random_case", func(t *testing.T) {
		price := Price(nil, nil)
		cci := CCI(price, 16)
		for _, value := range input {
			price.PushAndEmit(value)
		}

		t.Logf("cci: %+v", cci.Slice)

		last := cci.Last(0)
		assert.InDelta(t, 111.111111, last, delta)
		assert.InDelta(t, 97.323601, cci.Index(1), delta)
		assert.Equal(t, 35, cci.Length(), "length")
	})
}
