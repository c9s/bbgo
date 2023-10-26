package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestDojiGraveStone(t *testing.T) {
	hasPattern := []types.KLine{
		{Open: n(30.10), Low: n(30.12), High: n(36.13), Close: n(30.13)},
	}

	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := DojiGraveStone(kLines, 0.05)

	for _, candle := range hasPattern {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestDojiGraveStone Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	noPattern := []types.KLine{
		{Open: n(30.10), Low: n(30.11), High: n(30.10), Close: n(30.09)},
	}
	ind = DojiGraveStone(kLines, 0.01)

	for _, candle := range noPattern {
		stream.EmitKLineClosed(candle)
	}

	if ind.Last(0) == expectedBear {
		t.Errorf("TestDojiGraveStone Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}
}
