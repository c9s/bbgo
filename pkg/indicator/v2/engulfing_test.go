package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

func TestEngulfing(t *testing.T) {
	ts := []types.KLine{
		{Open: n(80), Low: n(75), High: n(95), Close: n(90)},
		{Open: n(100), Low: n(65), High: n(105), Close: n(70)},
	}
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := Engulfing(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestEngulfing Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: n(90), Low: n(75), High: n(95), Close: n(80)},
		{Open: n(70), Low: n(65), High: n(105), Close: n(100)},
	}
	ind = Engulfing(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestEngulfing Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
