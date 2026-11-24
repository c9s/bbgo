package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestNegativeVolumeIndex(t *testing.T) {
	ts := []types.KLine{
		{Volume: n(100), Close: n(9)},
		{Volume: n(110), Close: n(11)},
		{Volume: n(80), Close: n(7)},
		{Volume: n(120), Close: n(10)},
		{Volume: n(90), Close: n(8)},
	}

	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := NegativeVolumeIndex(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}

	assert.InDelta(t, 509.09, ind.Last(0), 0.01)
	assert.InDelta(t, 636.36, ind.Last(1), 0.01)
	assert.InDelta(t, 636.36, ind.Last(2), 0.01)
	assert.InDelta(t, 1000.0, ind.Last(3), 0.01)
	assert.InDelta(t, 1000.0, ind.Last(4), 0.01)

}
