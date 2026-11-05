package indicatorv2

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

/*
import pandas as pd
import pandas_ta as ta

data = pd.Series([0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9])
out = ta.psar(data, data)
print(out)
*/

func Test_PSAR(t *testing.T) {
	t.Run("legacy_case", func(t *testing.T) {
		var input []fixedpoint.Value
		err := json.Unmarshal([]byte(`[0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9]`), &input)
		require.NoError(t, err)

		var kLines []types.KLine
		for _, v := range input {
			kLines = append(kLines, types.KLine{High: v, Low: v})
		}

		stream := &KLineStream{}
		psar := PSAR(stream, 2)
		stream.BackFill(kLines)

		assert.Equal(t, 29, psar.Length())
		assert.Equal(t, 0.04, psar.AF)
		assert.Equal(t, 0.16, psar.Last(0))
	})

	for _, window := range []int{2, 3, 5} {
		t.Run(fmt.Sprintf("compare_with_legacy_window_%d", window), func(t *testing.T) {
			kLines := randomWalkKLines(300, 2)

			stream := &KLineStream{}
			psar := PSAR(stream, window)
			stream.BackFill(kLines)

			legacy := &indicator.PSAR{IntervalWindow: types.IntervalWindow{Window: window}}
			for _, k := range kLines {
				legacy.PushK(k)
			}

			assert.Equal(t, legacy.Length(), psar.Length())
			assert.InDeltaSlice(t, []float64(legacy.Values), []float64(psar.Slice), 1e-9)
			assert.Equal(t, legacy.AF, psar.AF)
			assert.Equal(t, legacy.EP, psar.EP)
			assert.Equal(t, legacy.Falling, psar.Falling)
		})
	}
}
