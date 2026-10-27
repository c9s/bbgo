package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

/*
import pandas as pd

data = pd.Series([0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9])
ma1 = data.diff(1).ewm(span=25, adjust=False).mean()
ma2 = ma1.ewm(span=13, adjust=False).mean()
ma3 = data.diff(1).abs().ewm(span=25, adjust=False).mean()
ma4 = ma3.ewm(span=13, adjust=False).mean()
print(ma2/ma4*100.)
*/
func Test_TSI(t *testing.T) {
	source := types.NewFloat64Series()
	tsi := TSI(source, 0, 0)

	for i := 0; i < 30; i++ {
		source.PushAndEmit(float64(i % 10))
	}

	// the first input is only used as the previous value
	assert.Equal(t, 29, tsi.Length())
	assert.InDelta(t, 22.89, tsi.Last(0), 1.5e-2)
}

func Test_TSI_Legacy(t *testing.T) {
	source := types.NewFloat64Series()
	tsi := TSI(source, 7, 15)
	legacy := &indicator.TSI{FastWindow: 7, SlowWindow: 15}

	for i := 0; i < 200; i++ {
		// a monotonic drift avoids zero price changes, see the EWMA2 zero-value warm-up note in tsi.go
		v := 100 + float64(i%10) + float64(i%7)*0.3 + float64(i)*0.01
		source.PushAndEmit(v)
		legacy.Update(v)
		assert.Equal(t, legacy.Length(), tsi.Length())
		if legacy.Length() > 0 {
			assert.InDelta(t, legacy.Last(0), tsi.Last(0), 1e-9)
		}
	}
}
