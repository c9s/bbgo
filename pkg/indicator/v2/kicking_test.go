package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

func TestKicking(t *testing.T) {
	ts := []types.KLine{
		{Open: n(100), Low: n(100), High: n(120), Close: n(120)},
		{Open: n(90), Low: n(70), High: n(90), Close: n(70)},
	}
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := Kicking(kLines, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestKicking Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: n(90), Low: n(70), High: n(90), Close: n(70)},
		{Open: n(100), Low: n(100), High: n(120), Close: n(120)},
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
