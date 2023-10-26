package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestPiercingLine(t *testing.T) {
	ts := []types.KLine{
		{Open: n(42.70), Low: n(41.45), High: n(42.82), Close: n(41.60)},
		{Open: n(41.33), Low: n(41.15), High: n(42.50), Close: n(42.34)},
	}

	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := PiercingLine(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestPiercingLine Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
