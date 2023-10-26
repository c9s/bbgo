package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestBelthold(t *testing.T) {

	ts := []types.KLine{
		{Open: n(60), Low: n(55), High: n(75), Close: n(70)},
		{Open: n(100), Low: n(75), High: n(100), Close: n(80)},
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
		{Open: n(120), Low: n(100), High: n(125), Close: n(105)},
		{Open: n(70), Low: n(70), High: n(95), Close: n(90)},
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
