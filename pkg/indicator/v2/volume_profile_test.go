package indicatorv2

import (
	"encoding/csv"
	"os"
	"path"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/datasource/csvsource"
	"github.com/c9s/bbgo/pkg/types"
)

func TestVolumeProfile(t *testing.T) {
	file, _ := os.Open(path.Join("testdata", "BTCUSDT-1m-2022-05-06.csv"))
	defer func() {
		assert.NoError(t, file.Close())
	}()

	candles, err := csvsource.NewCSVKLineReader(csv.NewReader(file)).ReadAll(time.Minute)
	assert.NoError(t, err)

	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := NewVolumeProfile(kLines, 10)

	for _, candle := range candles {
		stream.EmitKLineClosed(candle)
	}
	spew.Dump(ind)

	assert.InDelta(t, 35359.26666666667, ind.VP.Low, 0.01, "VP.LOW")
	assert.InDelta(t, 35569.12777777779, ind.VP.VAL, 0.01, "VP.VAL")
	assert.InDelta(t, 35988.850000000006, ind.VP.POC, 0.01, "VP.POC")
	assert.InDelta(t, 36408.572222222225, ind.VP.VAH, 0.01, "VP.VAH")
	assert.InDelta(t, 36617.433333333334, ind.VP.High, 0.01, "VP.HIGH")
}
