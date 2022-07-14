package indicator

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

// data from https://school.stockcharts.com/doku.php?id=technical_indicators:ease_of_movement_emv
func Test_EMV(t *testing.T) {
	var Delta = 0.01
	emv := &EMV{
		EMVScale:       100000000,
		IntervalWindow: types.IntervalWindow{Window: 14},
	}
	emv.Update(63.74, 62.63, 32178836)
	emv.Update(64.51, 63.85, 36461672)
	assert.InDelta(t, 1.8, emv.Values.rawValues.Last(), Delta)
	emv.Update(64.57, 63.81, 51372680)
	emv.Update(64.31, 62.62, 42476356)
	emv.Update(63.43, 62.73, 29504176)
	emv.Update(62.85, 61.95, 33098600)
	emv.Update(62.70, 62.06, 30577960)
	emv.Update(63.18, 62.69, 35693928)
	emv.Update(62.47, 61.54, 49768136)
	emv.Update(64.16, 63.21, 44759968)
	emv.Update(64.38, 63.87, 33425504)
	emv.Update(64.89, 64.29, 15895085)
	emv.Update(65.25, 64.48, 37015388)
	emv.Update(64.69, 63.65, 40672116)
	emv.Update(64.26, 63.68, 35627200)
	assert.InDelta(t, -0.03, emv.Last(), Delta)
}
