package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestKdj(t *testing.T) {
	ts := []types.KLine{
		{Low: n(1), High: n(10), Close: n(5)},
		{Low: n(2), High: n(20), Close: n(10)},
		{Low: n(3), High: n(30), Close: n(15)},
		{Low: n(4), High: n(40), Close: n(20)},
		{Low: n(5), High: n(50), Close: n(25)},
		{Low: n(6), High: n(60), Close: n(30)},
		{Low: n(7), High: n(70), Close: n(35)},
		{Low: n(8), High: n(80), Close: n(40)},
		{Low: n(9), High: n(90), Close: n(45)},
		{Low: n(10), High: n(100), Close: n(50)},
	}
	// expectedK := []float64{44.44, 45.91, 46.70, 48.12, 48.66, 48.95, 49.14, 49.26, 49.36, 49.26}
	// expectedD := []float64{44.44, 45.18, 45.68, 46.91, 47.82, 48.58, 48.91, 49.12, 49.25, 49.30}
	expectedJ := []float64{44.44, 47.37, 48.72, 50.55, 50.32, 49.70, 49.58, 49.56, 49.57, 49.19}

	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := KDJDefault(kLines)
	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	for i, v := range expectedJ {
		assert.InDelta(t, v, ind.Slice[i], 0.01, "Expected KDJ.slice[%d] to be %v, but got %v", i, v, ind.Slice[i])
	}
}
