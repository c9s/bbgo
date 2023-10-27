package volume

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestVolumeWeightedAveragePrice(t *testing.T) {
	ts := []types.KLine{
		{Volume: n(100), Close: n(9)},
		{Volume: n(110), Close: n(11)},
		{Volume: n(80), Close: n(7)},
		{Volume: n(120), Close: n(10)},
		{Volume: n(90), Close: n(8)},
	}
	expected := []float64{9, 10.05, 9.32, 8.8, 9.14}

	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := VWAP(kLines, 2)
	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	spew.Dump(ind)
	for i, v := range expected {
		assert.InDelta(t, v, ind.Slice[i], 0.01, "Expected VWAP.slice[%d] to be %v, but got %v", i, v, ind.Slice[i])
	}
}
