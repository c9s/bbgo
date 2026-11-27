package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

func TestDarkCloud(t *testing.T) {
	ts := []types.KLine{
		{Open: n(30.10), Low: n(28.30), High: n(37.40), Close: n(35.36)},
		{Open: n(39.45), Low: n(31.25), High: n(41.45), Close: n(32.50)},
	}

	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := DarkCloud(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestDarkCloud Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}
}
