package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestBreakAway(t *testing.T) {
	ts := []types.KLine{
		{Open: 70, Low: 60, High: 85, Close: 80},
		{Open: 115, Low: 110, High: 125, Close: 120},
		{Open: 120, Low: 115, High: 130, Close: 125},
		{Open: 125, Low: 120, High: 135, Close: 130},
		{Open: 125, Low: 95, High: 130, Close: 100},
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
		{Open: 130, Low: 115, High: 135, Close: 120},
		{Open: 100, Low: 85, High: 105, Close: 90},
		{Open: 95, Low: 80, High: 100, Close: 85},
		{Open: 90, Low: 75, High: 95, Close: 80},
		{Open: 85, Low: 80, High: 115, Close: 110},
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
