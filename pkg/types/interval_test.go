package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseInterval(t *testing.T) {
	assert.Equal(t, ParseInterval("1s"), 1)
	assert.Equal(t, ParseInterval("3m"), 3*60)
	assert.Equal(t, ParseInterval("15h"), 15*60*60)
	assert.Equal(t, ParseInterval("72d"), 72*24*60*60)
	assert.Equal(t, ParseInterval("3Mo"), 3*30*24*60*60)
}

func TestIntervalSort(t *testing.T) {
	intervals := IntervalSlice{Interval2h, Interval1m, Interval1h, Interval1d}
	intervals.Sort()
	assert.Equal(t, IntervalSlice{
		Interval1m,
		Interval1h,
		Interval2h,
		Interval1d,
	}, intervals)
}
