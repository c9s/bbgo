package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestSideBySideWhiteLines(t *testing.T) {
	ts := []types.KLine{
		{Open: n(130), Low: n(110), High: n(140), Close: n(120)},
		{Open: n(110), Low: n(85), High: n(115), Close: n(90)},
		{Open: n(50), Low: n(45), High: n(75), Close: n(70)},
		{Open: n(50), Low: n(42), High: n(77), Close: n(68)},
	}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := SideBySideWhiteLines(kLines, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestSideBySideWhiteLines Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: n(70), Low: n(60), High: n(90), Close: n(80)},
		{Open: n(100), Low: n(90), High: n(130), Close: n(120)},
		{Open: n(150), Low: n(140), High: n(210), Close: n(185)},
		{Open: n(150), Low: n(135), High: n(200), Close: n(190)},
	}
	ind = SideBySideWhiteLines(kLines, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestSideBySideWhiteLines Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
