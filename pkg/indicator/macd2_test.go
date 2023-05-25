package indicator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestSubtract(t *testing.T) {
	stream := &types.StandardStream{}
	kLines := KLines(stream)
	closePrices := ClosePrices(kLines)
	fastEMA := EWMA2(closePrices, 10)
	slowEMA := EWMA2(closePrices, 25)
	subtract := Subtract(fastEMA, slowEMA)

	for i := .0; i < 50.0; i++ {
		stream.EmitKLineClosed(types.KLine{Close: fixedpoint.NewFromFloat(19_000.0 + i)})
	}

	t.Logf("fastEMA: %+v", fastEMA.slice)
	t.Logf("slowEMA: %+v", slowEMA.slice)

	assert.Equal(t, len(subtract.a), len(subtract.b))
	assert.Equal(t, len(subtract.a), len(subtract.c))
	assert.Equal(t, subtract.c[0], subtract.a[0]-subtract.b[0])

	t.Logf("subtract.a: %+v", subtract.a)
	t.Logf("subtract.b: %+v", subtract.b)
	t.Logf("subtract.c: %+v", subtract.c)
}
