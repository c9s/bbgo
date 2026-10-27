package indicatorv2

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/marketdata/sources/binancecsv"
	"github.com/c9s/bbgo/pkg/types"
)

func TestVolumeProfile(t *testing.T) {
	candles, err := binancecsv.ReadKLineFile(
		path.Join("testdata", "BTCUSDT-1m-2022-05-06.csv"), "BTCUSDT", types.Interval1m)
	assert.NoError(t, err)
	assert.NotEmptyf(t, candles, "candles should not be empty")

	stream := &types.StandardStream{}
	kLines := KLines(stream, "BTCUSDT", types.Interval1m)
	ind := NewVolumeProfile(kLines, 10)

	for _, candle := range candles {
		t.Logf("emitting kline: %s", candle.String())
		stream.EmitKLineClosed(candle)
	}

	t.Logf("VP.LOW: %v", ind.VP.Low)
	t.Logf("VP.VAL: %v", ind.VP.VAL)
	t.Logf("VP.POC: %v", ind.VP.POC)
	t.Logf("VP.VAH: %v", ind.VP.VAH)
	t.Logf("VP.HIGH: %v", ind.VP.High)

	assert.InDelta(t, 36512.7, ind.VP.Low, 0.01, "VP.LOW")
	assert.InDelta(t, 36600.811111108334, ind.VP.VAL, 0.01, "VP.VAL")
	assert.InDelta(t, 36612.559259256115, ind.VP.POC, 0.01, "VP.POC")
	assert.InDelta(t, 36618.43333333, ind.VP.VAH, 0.01, "VP.VAH")
	assert.InDelta(t, 36617.433, ind.VP.High, 0.01, "VP.HIGH")
}

func TestVolumeProfile_EmptyBinsDoNotHang(t *testing.T) {
	// the point of control is the last non-empty bin and the bins below it have no volume,
	// so the value area can not grow on either side
	kLines := []types.KLine{
		{High: fixedpoint.NewFromFloat(100), Low: fixedpoint.NewFromFloat(100), Close: fixedpoint.NewFromFloat(100), Volume: fixedpoint.NewFromFloat(40)},
		{High: fixedpoint.NewFromFloat(100.2), Low: fixedpoint.NewFromFloat(100.2), Close: fixedpoint.NewFromFloat(100.2), Volume: fixedpoint.NewFromFloat(60)},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		stream := &KLineStream{}
		NewVolumeProfile(stream, 2)
		stream.BackFill(kLines)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("volume profile calculation does not terminate")
	}
}
