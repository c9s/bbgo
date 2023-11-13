package bitget

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_hasMaxDuration(t *testing.T) {
	ok, duration := hasMaxDuration(types.Interval1m)
	assert.True(t, ok)
	assert.Equal(t, 30*24*time.Hour, duration)

	ok, duration = hasMaxDuration(types.Interval5m)
	assert.True(t, ok)
	assert.Equal(t, 30*24*time.Hour, duration)

	ok, duration = hasMaxDuration(types.Interval15m)
	assert.True(t, ok)
	assert.Equal(t, 52*24*time.Hour, duration)

	ok, duration = hasMaxDuration(types.Interval30m)
	assert.True(t, ok)
	assert.Equal(t, 62*24*time.Hour, duration)

	ok, duration = hasMaxDuration(types.Interval1h)
	assert.True(t, ok)
	assert.Equal(t, 83*24*time.Hour, duration)

	ok, duration = hasMaxDuration(types.Interval4h)
	assert.True(t, ok)
	assert.Equal(t, 240*24*time.Hour, duration)

	ok, duration = hasMaxDuration(types.Interval6h)
	assert.True(t, ok)
	assert.Equal(t, 360*24*time.Hour, duration)
}
