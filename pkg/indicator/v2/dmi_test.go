package indicatorv2

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

/*
python:

import pandas as pd
import pandas_ta as ta

high = pd.Series([100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109])

low = pd.Series([80,81,82,83,84,85,86,87,88,89,80,81,82,83,84,85,86,87,88,89,80,81,82,83,84,85,86,87,88,89])

close = pd.Series([90,91,92,93,94,95,96,97,98,99,90,91,92,93,94,95,96,97,98,99,90,91,92,93,94,95,96,97,98,99])

result = ta.adx(high, low, close, 5, 14)
print(result['ADX_14'])

print(result['DMP_5'])
print(result['DMN_5'])
*/
func Test_DMI(t *testing.T) {
	var highs, lows, closes []fixedpoint.Value
	require.NoError(t, json.Unmarshal([]byte(`[100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109]`), &highs))
	require.NoError(t, json.Unmarshal([]byte(`[80,81,82,83,84,85,86,87,88,89,80,81,82,83,84,85,86,87,88,89,80,81,82,83,84,85,86,87,88,89]`), &lows))
	require.NoError(t, json.Unmarshal([]byte(`[90,91,92,93,94,95,96,97,98,99,90,91,92,93,94,95,96,97,98,99,90,91,92,93,94,95,96,97,98,99]`), &closes))

	var kLines []types.KLine
	for i := range highs {
		kLines = append(kLines, types.KLine{High: highs[i], Low: lows[i], Close: closes[i]})
	}

	t.Run("legacy_case", func(t *testing.T) {
		stream := &KLineStream{}
		dmi := DMI(stream, 5, 14)
		stream.BackFill(kLines)

		const delta = 0.001
		assert.InDelta(t, 4.85114, dmi.DIPlus.Last(0), delta)
		assert.InDelta(t, 1.339736, dmi.DIMinus.Last(0), delta)
		assert.InDelta(t, 37.857156, dmi.Last(0), delta)
		assert.InDelta(t, 37.857156, dmi.GetADX().Last(0), delta)
	})

	t.Run("compare_with_legacy", func(t *testing.T) {
		kLines := randomWalkKLines(300, 3)

		stream := &KLineStream{}
		dmi := DMI(stream, 7, 10)
		stream.BackFill(kLines)

		legacy := &indicator.DMI{IntervalWindow: types.IntervalWindow{Window: 7}, ADXSmoothing: 10}
		for _, k := range kLines {
			legacy.PushK(k)
		}

		require.Equal(t, legacy.GetADX().Length(), dmi.Length())
		require.Equal(t, legacy.GetDIPlus().Length(), dmi.DIPlus.Length())
		for i := 0; i < dmi.Length(); i++ {
			assert.InDelta(t, legacy.GetADX().Last(i), dmi.Last(i), 1e-9)
			assert.InDelta(t, legacy.GetDIPlus().Last(i), dmi.DIPlus.Last(i), 1e-9)
			assert.InDelta(t, legacy.GetDIMinus().Last(i), dmi.DIMinus.Last(i), 1e-9)
		}
	})
}

// Test_DMI_FlatMarketKeepsFinite verifies that candles with zero volatility do
// not poison the Wilder smoothing state.
func Test_DMI_FlatMarketKeepsFinite(t *testing.T) {
	stream := &KLineStream{}
	dmi := DMI(stream, 5, 14)

	flat := fixedpoint.NewFromFloat(100)
	for i := 0; i < 20; i++ {
		stream.BackFill([]types.KLine{{High: flat, Low: flat, Close: flat}})
	}

	assert.Equal(t, 0.0, dmi.Last(0))
	assert.Equal(t, 0.0, dmi.DIPlus.Last(0))
	assert.Equal(t, 0.0, dmi.DIMinus.Last(0))

	for i := 1; i <= 40; i++ {
		c := float64(100 + i)
		stream.BackFill([]types.KLine{{
			High:  fixedpoint.NewFromFloat(c + 1),
			Low:   fixedpoint.NewFromFloat(c - 1),
			Close: fixedpoint.NewFromFloat(c),
		}})
	}

	adx := dmi.Last(0)
	assert.False(t, math.IsNaN(adx))
	assert.False(t, math.IsInf(dmi.DIPlus.Last(0), 0))
	assert.False(t, math.IsInf(dmi.DIMinus.Last(0), 0))
	assert.Greater(t, adx, 0.0)
	assert.LessOrEqual(t, adx, 100.0)
}
