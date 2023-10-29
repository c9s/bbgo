package volatility

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestDonchianChannel(t *testing.T) {
	closing := []float64{9, 11, 7, 10, 8}
	expectedUpper := []float64{9, 11, 11, 11, 11}
	expectedMiddle := []float64{9, 10, 9, 9, 9}
	expectedLower := []float64{9, 9, 7, 7, 7}

	stream := &types.StandardStream{}
	ind := DonchianChannel(v2.KLines(stream, "", ""), 4)
	for i := range closing {
		stream.EmitKLineClosed(types.KLine{Close: fixedpoint.NewFromFloat(closing[i])})
	}
	for i, v := range expectedLower {
		assert.InDelta(t, v, ind.DownBand.Slice[i], 0.01, "Expected DonchianDownBand.slice[%d] to be %v, but got %v", i, v, ind.DownBand.Slice[i])
	}
	for i, v := range expectedMiddle {
		assert.InDelta(t, v, ind.MiddleBand.Slice[i], 0.01, "Expected DonchianUpBand.slice[%d] to be %v, but got %v", i, v, ind.UpBand.Slice[i])
	}
	for i, v := range expectedUpper {
		assert.InDelta(t, v, ind.UpBand.Slice[i], 0.01, "Expected DonchianUpBand.slice[%d] to be %v, but got %v", i, v, ind.UpBand.Slice[i])
	}
}
