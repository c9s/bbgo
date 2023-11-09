package indicatorv2

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_WilliamsR(t *testing.T) {
	high := []byte(`[127.0090,127.6159,126.5911,127.3472,128.1730,128.4317,127.3671,126.4220,126.8995,126.8498,125.6460,125.7156,127.1582,127.7154,127.6855,128.2228,128.2725,128.0934,128.2725,127.7353,128.7700,129.2873,130.0633,129.1182,129.2873,128.4715,128.0934,128.6506,129.1381,128.6406]`)
	low := []byte(`[125.3574,126.1633,124.9296,126.0937,126.8199,126.4817,126.0340,124.8301,126.3921,125.7156,124.5615,124.5715,125.0689,126.8597,126.6309,126.8001,126.7105,126.8001,126.1335,125.9245,126.9891,127.8148,128.4715,128.0641,127.6059,127.5960,126.9990,126.8995,127.4865,127.3970]`)
	close := []byte(`[125.3574,126.1633,124.9296,126.0937,126.8199,126.4817,126.0340,124.8301,126.3921,125.7156,124.5615,124.5715,125.0689,127.2876,127.1781,128.0138,127.1085,127.7253,127.0587,127.3273,128.7103,127.8745,128.5809,128.6008,127.9342,128.1133,127.5960,127.5960,128.6904,128.2725]`)
	buildKLines := func(high, low, close []fixedpoint.Value) (kLines []types.KLine) {
		for i := range high {
			kLines = append(kLines, types.KLine{High: high[i], Low: low[i], Close: close[i]})
		}
		return kLines
	}
	var h, l, c []fixedpoint.Value
	_ = json.Unmarshal(high, &h)
	_ = json.Unmarshal(low, &l)
	_ = json.Unmarshal(close, &c)

	expected := []float64{
		-29.561779752984485,
		-32.391090899695165,
		-10.797891581830443,
		-34.189447573768696,
		-18.252286703529535,
		-35.476202780218095,
		-25.470223659391287,
		-1.4185576808844131,
		-29.89546743408507,
		-26.943909266058082,
		-26.582209458722687,
		-38.76870971266242,
		-39.04372897645341,
		-59.61389774813938,
		-59.61389774813938,
		-33.171450662027304,
		-43.26858026481079,
	}
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := WilliamsR(kLines, 14)
	k := buildKLines(h, l, c)
	for _, candle := range k {
		stream.EmitKLineClosed(candle)
	}
	slices.Reverse(expected)
	for i, v := range expected {
		assert.InDelta(t, v, ind.Last(i), 0.000001, "Expected williamsR.slice[%d] to be %v, but got %v", i, v, ind.Slice[i])
	}
}
