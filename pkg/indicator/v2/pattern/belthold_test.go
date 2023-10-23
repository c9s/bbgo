package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestBelthold(t *testing.T) {

	ts := []types.KLine{
		{Open: 60, Low: 55, High: 75, Close: 70},
		{Open: 100, Low: 75, High: 100, Close: 80},
	}

	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := Belthold(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestBelthold Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: 120, Low: 100, High: 125, Close: 105},
		{Open: 70, Low: 70, High: 95, Close: 90},
	}

	ind = Belthold(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestBelthold Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
