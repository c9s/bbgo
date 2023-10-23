package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestDojiLongLegged(t *testing.T) {
	ts := []types.KLine{
		{Open: 85, Low: 80, High: 95, Close: 90},
		{Open: 95, Low: 90, High: 105, Close: 100},
		{Open: 105, Low: 100, High: 115, Close: 110},
		{Open: 170, Low: 120, High: 210, Close: 160},
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
		{Open: 90, Low: 80, High: 95, Close: 85},
		{Open: 100, Low: 90, High: 105, Close: 95},
		{Open: 110, Low: 100, High: 115, Close: 105},
		{Open: 160, Low: 120, High: 210, Close: 170},
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
