package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestHammerStick(t *testing.T) {
	ts := []types.KLine{
		{Open: n(30.10), Low: n(10.06), High: n(30.10), Close: n(26.13)},
	}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := HammerStick(kLines, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestHammerStick Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: n(26.13), Low: n(30.10), High: n(30.10), Close: n(10.06)},
	}
	ind = HammerStick(kLines, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestHammerStick Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}

func TestHammerStickInverted(t *testing.T) {
	ts := []types.KLine{
		{Open: n(30.10), Low: n(26.13), High: n(52.06), Close: n(26.13)},
	}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := HammerStick(kLines, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestHammerStick Inverted Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: n(26.13), Low: n(30.10), High: n(52.06), Close: n(30.10)},
	}
	ind = HammerStick(kLines, 0.01)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := -1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestHammerStick Inverted Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
