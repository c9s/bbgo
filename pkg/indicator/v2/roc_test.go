package indicatorv2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_ROC2(t *testing.T) {
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	roc := ROC2(kLines, 3)

	prices := []float64{10.0, 12.0, 11.0, 15.0, 18.0, 14.0}
	for i, p := range prices {
		stream.EmitKLineClosed(types.KLine{
			Close:   fixedpoint.NewFromFloat(p),
			EndTime: types.Time(time.Now().Add(time.Duration(i) * time.Minute)),
		})
	}

	assert.Equal(t, 3, roc.Length())
	assert.InDelta(t, 50.0, roc.Index(2), 0.0001)
	assert.InDelta(t, 50.0, roc.Index(1), 0.0001)
	assert.InDelta(t, 27.2727, roc.Index(0), 0.001)
	assert.InDelta(t, 27.2727, roc.Last(0), 0.001)
}

func Test_ROC2_zero_denominator(t *testing.T) {
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	roc := ROC2(kLines, 1)

	stream.EmitKLineClosed(types.KLine{Close: fixedpoint.NewFromFloat(0)})
	stream.EmitKLineClosed(types.KLine{Close: fixedpoint.NewFromFloat(10)})

	assert.Equal(t, 1, roc.Length())
	assert.InDelta(t, 0.0, roc.Last(0), 0.0001)
}
