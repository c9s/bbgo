package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestThreeLineStrike(t *testing.T) {
	ts := []types.KLine{
		{Open: 98, Low: 77, High: 100, Close: 80},
		{Open: 90, Low: 68, High: 95, Close: 73},
		{Open: 82, Low: 65, High: 86, Close: 67},
		{Open: 62, Low: 59, High: 103, Close: 101},
	}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := ThreeLineStrike(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestThreeLineStrike Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: 70, Low: 60, High: 100, Close: 90},
		{Open: 80, Low: 75, High: 110, Close: 105},
		{Open: 95, Low: 93, High: 120, Close: 115},
		{Open: 125, Low: 50, High: 130, Close: 55},
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
