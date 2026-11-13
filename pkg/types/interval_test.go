package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTruncate(t *testing.T) {
	ts := time.Date(2023, 11, 5, 17, 36, 43, 716, time.UTC)
	expectedDay := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedDay, Interval1d.Truncate(ts))
	expected2h := time.Date(ts.Year(), ts.Month(), ts.Day(), 16, 0, 0, 0, time.UTC)
	assert.Equal(t, expected2h, Interval2h.Truncate(ts))
	expectedHour := time.Date(ts.Year(), ts.Month(), ts.Day(), 17, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedHour, Interval1h.Truncate(ts))
	expected30m := time.Date(ts.Year(), ts.Month(), ts.Day(), 17, 30, 0, 0, time.UTC)
	assert.Equal(t, expected30m, Interval30m.Truncate(ts))

}

func TestTruncateMonthly(t *testing.T) {
	for _, ts := range []time.Time{
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 2, 29, 17, 36, 43, 716, time.UTC),
		time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC),
		time.Date(2023, 11, 30, 23, 59, 59, 999, time.UTC),
	} {
		t.Run(ts.Format(time.RFC3339Nano), func(t *testing.T) {
			expected := time.Date(ts.Year(), ts.Month(), 1, 0, 0, 0, 0, time.UTC)
			actual := Interval1mo.Truncate(ts)
			assert.Equal(t, expected, actual)
			assert.Equal(t, actual, Interval1mo.Truncate(actual))
		})
	}
}

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
