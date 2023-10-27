package volume

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestChaikinMoneyFlow(t *testing.T) {
	ts := []types.KLine{
		{Volume: n(100), Low: n(6), High: n(10), Close: n(9)},
		{Volume: n(110), Low: n(7), High: n(9), Close: n(11)},
		{Volume: n(80), Low: n(9), High: n(12), Close: n(7)},
		{Volume: n(120), Low: n(12), High: n(14), Close: n(10)},
		{Volume: n(90), Low: n(10), High: n(12), Close: n(8)},
	}
	expected := []float64{0.5, 1.81, 0.67, -0.41, -0.87}

	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := ChaikinMoneyFlowDefault(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}

	for i, v := range expected {
		assert.InDelta(t, v, ind.Slice[i], 0.5, "Expected AccumulationDistribution.slice[%d] to be %v, but got %v", i, v, ind.Slice[i])
	}
}
