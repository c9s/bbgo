package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestHaramiCross(t *testing.T) {
	ts := []types.KLine{
		{Open: n(20.12), Low: n(19.88), High: n(23.82), Close: n(23.50)},
		{Open: n(22.13), Low: n(21.31), High: n(22.76), Close: n(22.13)},
	}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := HaramiCross(kLines, Bearish, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestHaramiCross Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: n(25.13), Low: n(21.7), High: n(25.80), Close: n(22.14)},
		{Open: n(23.45), Low: n(23.07), High: n(24.59), Close: n(23.45)},
	}

	ind = HaramiCross(kLines, Bullish, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestHaramiCross Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
