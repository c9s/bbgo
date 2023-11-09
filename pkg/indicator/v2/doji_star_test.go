package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

func TestDojiStar(t *testing.T) {
	ts := []types.KLine{
		{Open: n(18.35), Low: n(18.13), High: n(21.60), Close: n(21.30)},
		{Open: n(22.20), Low: n(21.87), High: n(22.40), Close: n(22.22)},
		{Open: n(21.60), Low: n(19.30), High: n(22.05), Close: n(19.45)},
	}

	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := DojiStar(kLines, Bearish, 0.05)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestDojiEveningStar Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: n(22.20), Low: n(20.65), High: n(22.50), Close: n(20.80)},
		{Open: n(20.30), Low: n(20.10), High: n(20.45), Close: n(20.30)},
		{Open: n(20.70), Low: n(20.40), High: n(21.82), Close: n(21.58)},
	}
	stream = &types.StandardStream{}
	kLines = KLines(stream, "", "")
	ind = DojiStar(kLines, Bullish, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestDojiMorningStar Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
