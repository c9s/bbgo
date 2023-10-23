package pattern

import (
	"testing"

	"github.com/davecgh/go-spew/spew"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestThreeCrows(t *testing.T) {
	ts := []types.KLine{
		{Open: n(21.65), Low: n(21.25), High: n(21.82), Close: n(21.32)},
		{Open: n(21.48), Low: n(20.97), High: n(21.57), Close: n(21.10)},
		{Open: n(21.25), Low: n(20.60), High: n(21.35), Close: n(20.70)},
	}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := ThreeCrows(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	spew.Dump(ind)
	if ind.Last(0) != expectedBear {
		t.Errorf("TestThreeCrows Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}
}
