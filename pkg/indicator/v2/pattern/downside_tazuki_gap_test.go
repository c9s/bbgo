package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestTazukiGap(t *testing.T) {
	ts := []types.KLine{
		{Open: n(45.00), Low: n(38.56), High: n(46.20), Close: n(41.20)},
		{Open: n(33.45), Low: n(28), High: n(34.70), Close: n(29.31)},
		{Open: n(30.20), Low: n(29.80), High: n(36.63), Close: n(36.28)},
	}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := TazukiGap(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestTazukiGap Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}
}
