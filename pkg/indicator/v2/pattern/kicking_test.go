package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestKicking(t *testing.T) {
	ts := []types.KLine{
		{Open: 100, Low: 100, High: 120, Close: 120},
		{Open: 90, Low: 70, High: 90, Close: 70},
	}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := Kicking(kLines, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestKicking Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: 90, Low: 70, High: 90, Close: 70},
		{Open: 100, Low: 100, High: 120, Close: 120},
	}
	ind = Kicking(kLines, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestKicking Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
