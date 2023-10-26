package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestBreakAway(t *testing.T) {
	ts := []types.KLine{
		{Open: n(70), Low: n(60), High: n(85), Close: n(80)},
		{Open: n(115), Low: n(110), High: n(125), Close: n(120)},
		{Open: n(120), Low: n(115), High: n(130), Close: n(125)},
		{Open: n(125), Low: n(120), High: n(135), Close: n(130)},
		{Open: n(125), Low: n(95), High: n(130), Close: n(100)},
	}

	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := BreakAway(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestBreakAway Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: n(130), Low: n(115), High: n(135), Close: n(120)},
		{Open: n(100), Low: n(85), High: n(105), Close: n(90)},
		{Open: n(95), Low: n(80), High: n(100), Close: n(85)},
		{Open: n(90), Low: n(75), High: n(95), Close: n(80)},
		{Open: n(85), Low: n(80), High: n(115), Close: n(110)},
	}
	ind = BreakAway(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestBreakAway Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
