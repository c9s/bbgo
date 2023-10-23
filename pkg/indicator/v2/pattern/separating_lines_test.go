package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestSeparatingLines(t *testing.T) {
	ts := []types.KLine{
		{Open: 200, Low: 160, High: 210, Close: 170},
		{Open: 150, Low: 140, High: 190, Close: 180},
		{Open: 152, Low: 120, High: 152, Close: 130},
	}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := SeparatingLines(kLines, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestSeparatingLines Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: 50, Low: 40, High: 80, Close: 70},
		{Open: 100, Low: 70, High: 110, Close: 80},
		{Open: 102, Low: 102, High: 130, Close: 120},
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
