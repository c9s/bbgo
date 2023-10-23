package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestHarami(t *testing.T) {
	ts := []types.KLine{
		{Open: 100, Low: 95, High: 125, Close: 120},
		{Open: 110, Low: 100, High: 115, Close: 105},
	}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := Harami(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestHarami Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: 120, Low: 95, High: 125, Close: 100},
		{Open: 105, Low: 100, High: 115, Close: 110},
	}
	ind = Harami(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestHarami Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
