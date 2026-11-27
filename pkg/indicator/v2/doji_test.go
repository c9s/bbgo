package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

func TestDoji(t *testing.T) {
	ts := []types.KLine{
		{Open: n(30.10), Low: n(32.10), High: n(30.13), Close: n(28.10)},
	}

	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := Doji(kLines, 0.05)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	hasPattern := 1.0

	if ind.Last(0) == hasPattern {
		t.Errorf("TestDoji Bear unexpected result: got %v want %v", ind.Last(0), hasPattern)
	}

	ts = []types.KLine{
		{Open: n(30.10), Low: n(30.11), High: n(30.10), Close: n(30.09)},
	}
	ind = Doji(kLines, 0.05)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	hasPattern = 1.0

	if ind.Last(0) != hasPattern {
		t.Errorf("TestDoji Bull unexpected result: got %v want %v", ind.Last(0), hasPattern)
	}
}
