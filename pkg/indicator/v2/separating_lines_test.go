package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

func TestSeparatingLines(t *testing.T) {
	ts := []types.KLine{
		{Open: n(200), Low: n(160), High: n(210), Close: n(170)},
		{Open: n(150), Low: n(140), High: n(190), Close: n(180)},
		{Open: n(152), Low: n(120), High: n(152), Close: n(130)},
	}
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := SeparatingLines(kLines, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestSeparatingLines Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: n(50), Low: n(40), High: n(80), Close: n(70)},
		{Open: n(100), Low: n(70), High: n(110), Close: n(80)},
		{Open: n(102), Low: n(102), High: n(130), Close: n(120)},
	}
	ind = SeparatingLines(kLines, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestSeparatingLines Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
