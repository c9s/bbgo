package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestATRP2(t *testing.T) {
	stream := &KLineStream{}
	atr := ATR2(stream, 14)
	atrp := ATRP2(stream, 14)

	kLines := randomWalkKLines(100, 3)
	stream.BackFill(kLines)

	if assert.Equal(t, atr.Length(), atrp.Length()) {
		for i := 0; i < atrp.Length(); i++ {
			k := kLines[len(kLines)-1-i]
			assert.InDelta(t, atr.Last(i)/k.Close.Float64()*100, atrp.Last(i), 1e-9, "ATRP is the ATR percentage of the close price")
		}
	}
}
