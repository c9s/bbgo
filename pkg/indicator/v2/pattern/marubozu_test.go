package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestMarubozu(t *testing.T) {
	ts := []types.KLine{
		{Open: 200, Low: 100, High: 200, Close: 100},
	}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := Marubozu(kLines, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestMarubozu Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: 100, Low: 100, High: 200, Close: 200},
	}
	ind = Marubozu(kLines, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestMarubozu Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
