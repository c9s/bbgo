package trend

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestQstick(t *testing.T) {
	ts := []types.KLine{
		{Open: n(10), Close: n(20)},
		{Open: n(20), Close: n(15)},
		{Open: n(15), Close: n(50)},
		{Open: n(50), Close: n(55)},
		{Open: n(40), Close: n(42)},
		{Open: n(41), Close: n(30)},
		{Open: n(43), Close: n(31)},
		{Open: n(80), Close: n(70)},
	}
	expected := []float64{10, 2.5, 13.33, 11.25, 9.4, 5.2, 3.8, -5.2}

	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := Qstick(kLines, 5)
	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	for i, v := range expected {
		assert.InDelta(t, v, ind.Slice[i], 0.01, "Expected QStick.slice[%d] to be %v, but got %v", i, v, ind.Slice[i])
	}
}
