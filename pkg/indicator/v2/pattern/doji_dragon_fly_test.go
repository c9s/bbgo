package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestDojiDragonFly(t *testing.T) {
	ts := []types.KLine{
		{Open: n(30.10), Low: n(30.10), High: n(30.13), Close: n(28.10)},
	}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := DojiDragonFly(kLines)
	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestDojiDragonFly Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}
	ts = []types.KLine{
		{Open: n(30.10), Low: n(30.11), High: n(30.10), Close: n(30.09)},
	}

	ind = DojiDragonFly(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestDojiDragonFly Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
