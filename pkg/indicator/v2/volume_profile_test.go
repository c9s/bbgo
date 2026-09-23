package indicatorv2

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

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
