package pattern

import (
	"testing"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestMeetingLines(t *testing.T) {
	ts := []types.KLine{
		{Open: 85, Low: 85, High: 100, Close: 95},
		{Open: 95, Low: 90, High: 120, Close: 115},
		{Open: 130, Low: 105, High: 140, Close: 110},
	}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := MeetingLines(kLines)

	for _, candle := range ts {
		stream.EmitKLineClosed(candle)
	}
	expectedBear := -1.0

	if ind.Last(0) != expectedBear {
		t.Errorf("TestMeetingLines Bear unexpected result: got %v want %v", ind.Last(0), expectedBear)
	}

	ts = []types.KLine{
		{Open: 200, Low: 180, High: 210, Close: 190},
		{Open: 180, Low: 140, High: 195, Close: 150},
		{Open: 110, Low: 105, High: 160, Close: 155},
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
