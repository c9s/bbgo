package indicatorv2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestAccumulationDistribution(t *testing.T) {
	high := []byte(`[62.3400,62.0500,62.2700,60.7900,59.9300,61.7500,60.0000,59.0000,59.0700,59.2200,58.7500,58.6500,58.4700,58.2500,58.3500,59.8600,59.5299,62.1000,62.1600,62.6700,62.3800,63.7300,63.8500,66.1500,65.3400,66.4800,65.2300,63.4000,63.1800,62.7000]`)
	low := []byte(`[61.3700,60.6900,60.1000,58.6100,58.7120,59.8600,57.9700,58.0200,57.4800,58.3000,57.8276,57.8600,57.9100,57.8333,57.5300,58.5800,58.3000,58.5300,59.8000,60.9300,60.1500,62.2618,63.0000,63.5800,64.0700,65.2000,63.2100,61.8800,61.1100,61.2500]`)
	close := []byte(`[62.1500,60.8100,60.4500,59.1800,59.2400,60.2000,58.4800,58.2400,58.6900,58.6500,58.4700,58.0200,58.1700,58.0700,58.1300,58.9400,59.1000,61.9200,61.3700,61.6800,62.0900,62.8900,63.5300,64.0100,64.7700,65.2200,63.2800,62.4000,61.5500,62.6900]`)
	volume := []byte(`[7849.025,11692.075,10575.307,13059.128,20733.508,29630.096,17705.294,7259.203,10474.629,5203.714,3422.865,3962.15,4095.905,3766.006,4239.335,8039.979,6956.717,18171.552,22225.894,14613.509,12319.763,15007.69,8879.667,22693.812,10191.814,10074.152,9411.62,10391.69,8926.512,7459.575]`)
	buildKLines := func(high, low, close, volume []fixedpoint.Value) (kLines []types.KLine) {
		for i := range high {
			kLines = append(kLines, types.KLine{High: high[i], Low: low[i], Close: close[i], Volume: volume[i]})
		}
		return kLines
	}
	var h, l, c, v []fixedpoint.Value
	_ = json.Unmarshal(high, &h)
	_ = json.Unmarshal(low, &l)
	_ = json.Unmarshal(close, &c)
	_ = json.Unmarshal(volume, &v)

	expected := []float64{4774, -4855, -12019, -18249, -21006, -39976, -48785, -52785, -47317, -48561, -47216, -49574, -49866, -49354, -47389, -50907, -48813, -32474, -25128, -27144, -18028, -20193, -18000, -33099, -32056, -41816, -50575, -53856, -58988, -51631}
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	ind := AccumulationDistribution(kLines)
	k := buildKLines(h, l, c, v)
	for _, candle := range k {
		stream.EmitKLineClosed(candle)
	}
	for i, v := range expected {
		assert.InDelta(t, v, ind.Slice[i], 0.5, "Expected AccumulationDistribution.slice[%d] to be %v, but got %v", i, v, ind.Slice[i])
	}
}
