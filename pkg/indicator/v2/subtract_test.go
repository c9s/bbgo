package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_v2_Subtract(t *testing.T) {
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	closePrices := ClosePrices(kLines)
	toDiff := []DiffValue{
		{
			Minuend:    dec.New(10),
			Subtrahend: dec.New(8),
		},
		{
			Minuend:    dec.New(9),
			Subtrahend: dec.New(9),
		},
		{
			Minuend:    dec.New(8),
			Subtrahend: dec.New(10),
		},
	}

	diff := Subtract(fastEMA, slowEMA)

	for i := .0; i < 50.0; i++ {
		stream.EmitKLineClosed(types.KLine{Close: fixedpoint.NewFromFloat(19_000.0 + i)})
	}

	assert.Equal(t, []float64{2, 0, -2}, dec.FloatSlice(2, diff.Series()...))

	// t.Logf("fastEMA: %+v", fastEMA.Slice)
	// t.Logf("slowEMA: %+v", slowEMA.Slice)

	// assert.Equal(t, len(subtract.a), len(subtract.b))
	// assert.Equal(t, len(subtract.a), len(subtract.Slice))
	// assert.InDelta(t, subtract.Slice[0], subtract.a[0]-subtract.b[0], 0.0001)
}
