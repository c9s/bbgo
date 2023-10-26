package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestSpinningTop(t *testing.T) {
	ts := []types.KLine{
		{Open: n(20.50), Low: n(20.23), High: n(20.87), Close: n(20.62)},
	}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := SpinningTop(kLines, Bearish)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestSpinningTop Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: n(20.62), Low: n(20.34), High: n(20.75), Close: n(20.50)},
	}
	ind = SpinningTop(kLines, Bullish)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestSpinningTop Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
