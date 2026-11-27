package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

func TestHarami(t *testing.T) {
	ts := []types.KLine{
		{Open: n(100), Low: n(95), High: n(125), Close: n(120)},
		{Open: n(110), Low: n(100), High: n(115), Close: n(105)},
	}
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := Harami(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestHarami Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: n(120), Low: n(95), High: n(125), Close: n(100)},
		{Open: n(105), Low: n(100), High: n(115), Close: n(110)},
	}
	ind = Harami(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestHarami Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
