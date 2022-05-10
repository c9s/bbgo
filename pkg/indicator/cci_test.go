package indicator

import (
	"encoding/json"
	"testing"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

/*
python:

import pandas as pd
s = pd.Series([0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9])
cci = pd.Series((s - s.rolling(16).mean()) / (0.015 * s.rolling(16).std(ddof=0)), name="CCI")
print(cci)
*/
func Test_CCI(t *testing.T) {
	var randomPrices = []byte(`[0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9]`)
	var input []float64
	var Delta = 4.3e-2
	if err := json.Unmarshal(randomPrices, &input); err != nil {
		panic(err)
	}
	t.Run("random_case", func(t *testing.T) {
		cci := CCI{IntervalWindow: types.IntervalWindow{Window: 16}}
		for _, value := range input {
			cci.Update(value)
		}

		last := cci.Last()
		assert.InDelta(t, 93.250481, last, Delta)
		assert.InDelta(t, 81.813449, cci.Index(1), Delta)
		assert.Equal(t, 50-16+1, cci.Length())
	})
}
