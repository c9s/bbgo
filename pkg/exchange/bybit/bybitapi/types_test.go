package bybitapi

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_SupportedIntervals(t *testing.T) {
	assert.Equal(t, SupportedIntervals[types.Interval1m], 60)
	assert.Equal(t, SupportedIntervals[types.Interval3m], 180)
	assert.Equal(t, SupportedIntervals[types.Interval5m], 300)
	assert.Equal(t, SupportedIntervals[types.Interval15m], 15*60)
	assert.Equal(t, SupportedIntervals[types.Interval30m], 30*60)
	assert.Equal(t, SupportedIntervals[types.Interval1h], 60*60)
	assert.Equal(t, SupportedIntervals[types.Interval2h], 60*60*2)
	assert.Equal(t, SupportedIntervals[types.Interval4h], 60*60*4)
	assert.Equal(t, SupportedIntervals[types.Interval6h], 60*60*6)
	assert.Equal(t, SupportedIntervals[types.Interval12h], 60*60*12)
	assert.Equal(t, SupportedIntervals[types.Interval1d], 60*60*24)
	assert.Equal(t, SupportedIntervals[types.Interval1w], 60*60*24*7)
	assert.Equal(t, SupportedIntervals[types.Interval1mo], 60*60*24*30)
}

func Test_ToGlobalInterval(t *testing.T) {
	assert.Equal(t, ToGlobalInterval["1"], types.Interval1m)
	assert.Equal(t, ToGlobalInterval["3"], types.Interval3m)
	assert.Equal(t, ToGlobalInterval["5"], types.Interval5m)
	assert.Equal(t, ToGlobalInterval["15"], types.Interval15m)
	assert.Equal(t, ToGlobalInterval["30"], types.Interval30m)
	assert.Equal(t, ToGlobalInterval["60"], types.Interval1h)
	assert.Equal(t, ToGlobalInterval["120"], types.Interval2h)
	assert.Equal(t, ToGlobalInterval["240"], types.Interval4h)
	assert.Equal(t, ToGlobalInterval["360"], types.Interval6h)
	assert.Equal(t, ToGlobalInterval["720"], types.Interval12h)
	assert.Equal(t, ToGlobalInterval["D"], types.Interval1d)
	assert.Equal(t, ToGlobalInterval["W"], types.Interval1w)
	assert.Equal(t, ToGlobalInterval["M"], types.Interval1mo)
}
