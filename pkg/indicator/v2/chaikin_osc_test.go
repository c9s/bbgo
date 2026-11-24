package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_ChaikinOscillator(t *testing.T) {
	ts := []types.KLine{
		{Volume: n(100), Low: n(1), Close: n(5), High: n(10)},
		{Volume: n(200), Low: n(2), Close: n(6), High: n(11)},
		{Volume: n(300), Low: n(3), Close: n(7), High: n(12)},
		{Volume: n(400), Low: n(4), Close: n(8), High: n(13)},
		{Volume: n(500), Low: n(5), Close: n(9), High: n(14)},
		{Volume: n(600), Low: n(6), Close: n(10), High: n(15)},
		{Volume: n(700), Low: n(7), Close: n(11), High: n(16)},
		{Volume: n(800), Low: n(8), Close: n(12), High: n(17)},
	}

	expected := []float64{0, -7.41, -18.52, -31.69, -46.09, -61.27, -76.95, -92.97}

	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := ChaikinOscillator(kLines, 2, 5)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	for i, v := range expected {
		assert.InDelta(t, v, ind.Slice[i], 0.01, "Expected chaikin_osc.slice[%d] to be %v, but got %v", i, v, ind.Slice[i])
	}
}
