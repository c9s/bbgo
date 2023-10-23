package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestSideBySideWhiteLines(t *testing.T) {
	ts := []types.KLine{
		{Open: 130, Low: 110, High: 140, Close: 120},
		{Open: 110, Low: 85, High: 115, Close: 90},
		{Open: 50, Low: 45, High: 75, Close: 70},
		{Open: 50, Low: 42, High: 77, Close: 68},
	}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := SideBySideWhiteLines(kLines, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestSideBySideWhiteLines Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: 70, Low: 60, High: 90, Close: 80},
		{Open: 100, Low: 90, High: 130, Close: 120},
		{Open: 150, Low: 140, High: 210, Close: 185},
		{Open: 150, Low: 135, High: 200, Close: 190},
	}
	ind = SideBySideWhiteLines(kLines, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestSideBySideWhiteLines Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
