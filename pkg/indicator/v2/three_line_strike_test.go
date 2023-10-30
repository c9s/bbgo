package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

func TestThreeLineStrike(t *testing.T) {
	ts := []types.KLine{
		{Open: n(98), Low: n(77), High: n(100), Close: n(80)},
		{Open: n(90), Low: n(68), High: n(95), Close: n(73)},
		{Open: n(82), Low: n(65), High: n(86), Close: n(67)},
		{Open: n(62), Low: n(59), High: n(103), Close: n(101)},
	}
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := ThreeLineStrike(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestThreeLineStrike Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: n(70), Low: n(60), High: n(100), Close: n(90)},
		{Open: n(80), Low: n(75), High: n(110), Close: n(105)},
		{Open: n(95), Low: n(93), High: n(120), Close: n(115)},
		{Open: n(125), Low: n(50), High: n(130), Close: n(55)},
	}
	ind = ThreeLineStrike(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestThreeLineStrike Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}

}
