package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

func TestDojiDragonFly(t *testing.T) {
	ts := []types.KLine{
		{Open: n(30.10), Low: n(28.10), High: n(30.13), Close: n(30.09)},
	}

	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := DojiDragonFly(kLines, 0.05)
	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0
	if ind.Last(0) != expectedBull {
		t.Errorf("TestDojiDragonFly Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
	ts = []types.KLine{
		{Open: n(30.10), Low: n(30.11), High: n(30.10), Close: n(30.09)},
	}

	ind = DojiDragonFly(kLines, 0.05)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull = 1.0

	if ind.Last(0) == expectedBull {
		t.Errorf("TestDojiDragonFly Not Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
