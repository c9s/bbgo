package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

func TestMeetingLines(t *testing.T) {
	ts := []types.KLine{
		{Open: n(85), Low: n(85), High: n(100), Close: n(95)},
		{Open: n(95), Low: n(90), High: n(120), Close: n(115)},
		{Open: n(130), Low: n(105), High: n(140), Close: n(110)},
	}
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := MeetingLines(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestMeetingLines Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: n(200), Low: n(180), High: n(210), Close: n(190)},
		{Open: n(180), Low: n(140), High: n(195), Close: n(150)},
		{Open: n(110), Low: n(105), High: n(160), Close: n(155)},
	}
	ind = MeetingLines(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBull := 1.0

	if ind.Last(0) != expectedBull {
		t.Errorf("TestMeetingLines Bull unexpected result: got %v want %v", ind.Last(0), expectedBull)
	}
}
