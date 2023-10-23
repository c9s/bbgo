package volume

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestNegativeVolumeIndex(t *testing.T) {
	ts := []types.KLine{
		{Volume: 100, Close: 9},
		{Volume: 110, Close: 11},
		{Volume: 80, Close: 7},
		{Volume: 120, Close: 10},
		{Volume: 90, Close: 8},
	}

	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := NegativeVolumeIndex(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	assert.InDelta(t, 509.09, ind.Last(0), 0.001)
	assert.InDelta(t, 636.36, ind.Last(1), 0.001)
	assert.InDelta(t, 636.36, ind.Last(2), 0.001)
	assert.InDelta(t, 1000.0, ind.Last(3), 0.001)
	assert.InDelta(t, 1000.0, ind.Last(4), 0.001)

}
