package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestAbandonedBaby(t *testing.T) {
	ts := []types.KLine{
		{Open: 90, Low: 85, High: 105, Close: 100},
		{Open: 125, Low: 120, High: 135, Close: 130},
		{Open: 110, Low: 92, High: 115, Close: 95},
	}

	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := AbondonedBaby(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestAbandonedBaby Bear unexpected result: got %v want %v", ind, expectedBear)
	}
}
