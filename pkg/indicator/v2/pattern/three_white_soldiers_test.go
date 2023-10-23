package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestThreeWhiteSoldiers(t *testing.T) {
	ts := []types.KLine{
		{Open: n(21.12), Low: n(20.85), High: n(21.83), Close: n(21.65)},
		{Open: n(21.48), Low: n(21.36), High: n(21.36), Close: n(22.20)},
		{Open: n(21.80), Low: n(21.66), High: n(21.66), Close: n(22.65)},
	}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := ThreeWhiteSoldiers(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestThreeWhiteSoldiers Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
