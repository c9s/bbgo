package trend

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestVortex(t *testing.T) {
	high := []float64{1404.14, 1405.95, 1405.98, 1405.87, 1410.03}
	low := []float64{1396.13, 1398.80, 1395.62, 1397.32, 1400.60}
	closing := []float64{1402.22, 1402.80, 1405.87, 1404.11, 1403.93}
	expectedPlusVi := []float64{0.00000, 1.37343, 0.97087, 1.04566, 1.12595}
	expectedMinusVi := []float64{0.00000, 0.74685, 0.89492, 0.93361, 0.83404}

	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := Vortex(kLines)
	var ts []types.KLine
	for i := range closing {
		kline := types.KLine{Low: n(low[i]), High: n(high[i]), Close: n(closing[i])}
		ts = append(ts, kline)
	}
	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	spew.Dump(ind)
	for i, v := range expectedPlusVi {
		assert.InDelta(t, v, ind.PlusVi.Slice[i], 0.01, "Expected Vortex.slice[%d] to be %v, but got %v", i, v, ind.PlusVi.Slice[i])
	}
	for i, v := range expectedMinusVi {
		assert.InDelta(t, v, ind.MinusVi.Slice[i], 0.01, "Expected Vortex.slice[%d] to be %v, but got %v", i, v, ind.MinusVi.Slice[i])
	}
}
