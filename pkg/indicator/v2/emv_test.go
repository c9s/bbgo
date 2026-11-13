package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// data from https://school.stockcharts.com/doku.php?id=technical_indicators:ease_of_movement_emv
func TestEMV(t *testing.T) {
	const delta = 0.01
	data := [][3]float64{
		{63.74, 62.63, 32178836},
		{64.51, 63.85, 36461672},
		{64.57, 63.81, 51372680},
		{64.31, 62.62, 42476356},
		{63.43, 62.73, 29504176},
		{62.85, 61.95, 33098600},
		{62.70, 62.06, 30577960},
		{63.18, 62.69, 35693928},
		{62.47, 61.54, 49768136},
		{64.16, 63.21, 44759968},
		{64.38, 63.87, 33425504},
		{64.89, 64.29, 15895085},
		{65.25, 64.48, 37015388},
		{64.69, 63.65, 40672116},
		{64.26, 63.68, 35627200},
	}

	var kLines []types.KLine
	for _, d := range data {
		kLines = append(kLines, types.KLine{
			High:   fixedpoint.NewFromFloat(d[0]),
			Low:    fixedpoint.NewFromFloat(d[1]),
			Volume: fixedpoint.NewFromFloat(d[2]),
		})
	}

	stream := &KLineStream{}
	emv := EMV(stream, 14, 100000000)

	stream.BackFill(kLines[:2])
	assert.InDelta(t, 1.8, emv.Raw.Last(0), delta)
	assert.Equal(t, 0, emv.Length())

	stream.BackFill(kLines[2:14])
	assert.Equal(t, 13, emv.Raw.Length())
	assert.Equal(t, 0, emv.Length())

	stream.BackFill(kLines[14:])
	assert.Equal(t, 1, emv.Length())
	assert.InDelta(t, -0.03, emv.Last(0), delta)
}

func TestEMV_DefaultScale(t *testing.T) {
	stream := &KLineStream{}
	emv := EMV(stream, 2, 0)
	stream.BackFill([]types.KLine{
		{High: fixedpoint.NewFromFloat(10), Low: fixedpoint.NewFromFloat(8), Volume: fixedpoint.NewFromFloat(1e8)},
		{High: fixedpoint.NewFromFloat(12), Low: fixedpoint.NewFromFloat(10), Volume: fixedpoint.NewFromFloat(1e8)},
		{High: fixedpoint.NewFromFloat(11), Low: fixedpoint.NewFromFloat(10), Volume: fixedpoint.NewFromFloat(2e8)},
	})

	// raw: (11 - 9) / (1 / 2) = 4, (10.5 - 11) / (2 / 1) = -0.25
	assert.InDeltaSlice(t, []float64{4, -0.25}, []float64(emv.Raw.Slice), 1e-9)
	assert.Equal(t, 1, emv.Length())
	assert.InDelta(t, 1.875, emv.Last(0), 1e-9)
}
