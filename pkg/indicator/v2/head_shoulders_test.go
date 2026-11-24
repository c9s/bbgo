package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

func TestHeadShoulderSimple(t *testing.T) {
	ts := []types.KLine{
		{Open: n(90), Low: n(80), High: n(95), Close: n(90)},
		{Open: n(85), Low: n(75), High: n(90), Close: n(85)},
		{Open: n(80), Low: n(70), High: n(85), Close: n(80)},
		{Open: n(90), Low: n(80), High: n(95), Close: n(90)},
		{Open: n(85), Low: n(75), High: n(90), Close: n(85)},
		{Open: n(80), Low: n(70), High: n(85), Close: n(80)},
		{Open: n(75), Low: n(65), High: n(80), Close: n(75)},
		{Open: n(80), Low: n(70), High: n(85), Close: n(80)},
		{Open: n(85), Low: n(75), High: n(90), Close: n(85)},
		{Open: n(90), Low: n(80), High: n(95), Close: n(90)},
	}
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := HeadShoulderSimple(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(2) != expectedBear {
		t.Errorf("TestHeadShoulder unexpected result: got %v want %v", ind.Last(2), expectedBear)
	}
	inverse := []types.KLine{
		{Open: n(20), Low: n(15), High: n(25), Close: n(20)},
		{Open: n(25), Low: n(20), High: n(30), Close: n(25)},
		{Open: n(30), Low: n(25), High: n(35), Close: n(30)},
		{Open: n(20), Low: n(15), High: n(25), Close: n(20)},
		{Open: n(25), Low: n(20), High: n(30), Close: n(25)},
		{Open: n(30), Low: n(25), High: n(35), Close: n(30)},
		{Open: n(35), Low: n(30), High: n(40), Close: n(35)},
		{Open: n(30), Low: n(25), High: n(35), Close: n(30)},
		{Open: n(25), Low: n(20), High: n(30), Close: n(25)},
		{Open: n(20), Low: n(15), High: n(25), Close: n(20)},
	}
	for _, candle := range inverse {
		stream.EmitKLineClosed(candle)
	}

	expectedBull := 1.0

	if ind.Last(2) != expectedBull {
		t.Errorf("TestInverseHeadShoulder unexpected result: got %v want %v", ind.Last(2), expectedBull)
	}
}
