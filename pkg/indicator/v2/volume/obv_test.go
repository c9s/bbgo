package volume

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestOBV(t *testing.T) {
	close := []byte(`[53.26,53.30,53.32,53.72,54.19,53.92,54.65,54.60,54.21,54.53,53.79,53.66,53.56,53.57,53.94,53.27]`)
	volume := []byte(`[88888,8200,8100,8300,8900,9200,13300,10300,9900,10100,11300,12600,10700,11500,23800,14600]`)
	buildKLines := func(close, volume []fixedpoint.Value) (kLines []types.KLine) {
		for i := range close {
			kLines = append(kLines, types.KLine{Close: close[i], Volume: volume[i]})
		}
		return kLines
	}
	var c, v []fixedpoint.Value
	_ = json.Unmarshal(close, &c)
	_ = json.Unmarshal(volume, &v)

	expected := []float64{0, 8200, 16300, 24600, 33500, 24300, 37600, 27300, 17400, 27500, 16200, 3600, -7100, 4400, 28200, 13600}
	stream := &types.StandardStream{}
	kLines := v2.KLines(stream, "", "")
	ind := OBV(kLines)
	k := buildKLines(c, v)
	for _, candle := range k {
		stream.EmitKLineClosed(candle)
	}
	for i, v := range expected {
		assert.InDelta(t, v, ind.Slice[i], 0.01, "Expected OBV.slice[%d] to be %v, but got %v", i, v, ind.Slice[i])
	}
}
