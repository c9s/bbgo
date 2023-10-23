package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestEngulfing(t *testing.T) {
	ts := []types.KLine{
		{Open: 80, Low: 75, High: 95, Close: 90},
		{Open: 100, Low: 65, High: 105, Close: 70},
	}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := Engulfing(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestEngulfing Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: 90, Low: 75, High: 95, Close: 80},
		{Open: 70, Low: 65, High: 105, Close: 100},
	}
	ind = Engulfing(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestEngulfing Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
