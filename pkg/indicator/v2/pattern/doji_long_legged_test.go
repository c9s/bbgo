package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestDojiLongLegged(t *testing.T) {
	ts := []types.KLine{
		{Open: n(85), Low: n(80), High: n(95), Close: n(90)},
		{Open: n(95), Low: n(90), High: n(105), Close: n(100)},
		{Open: n(105), Low: n(100), High: n(115), Close: n(110)},
		{Open: n(170), Low: n(120), High: n(210), Close: n(160)},
	}

	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := DojiLongLegged(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestDojiLL Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: n(90), Low: n(80), High: n(95), Close: n(85)},
		{Open: n(100), Low: n(90), High: n(105), Close: n(95)},
		{Open: n(110), Low: n(100), High: n(115), Close: n(105)},
		{Open: n(160), Low: n(120), High: n(210), Close: n(170)},
	}
	ind = DojiLongLegged(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestDojiLL Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
