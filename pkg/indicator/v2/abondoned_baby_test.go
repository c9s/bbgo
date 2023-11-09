package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

func TestAbandonedBaby(t *testing.T) {
	ts := []types.KLine{
		{Open: n(90), Low: n(85), High: n(105), Close: n(100)},
		{Open: n(125), Low: n(120), High: n(135), Close: n(130)},
		{Open: n(110), Low: n(92), High: n(115), Close: n(95)},
	}

	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := AbondonedBaby(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestAbandonedBaby Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}
}
