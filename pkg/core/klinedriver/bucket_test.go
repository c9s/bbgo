package klinedriver

import (
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_findNextBucket(t *testing.T) {
	startTime := time.Now()
	bucket := &Bucket{
		StartTime: types.Time(startTime),
		EndTime:   types.Time(startTime.Add(time.Minute)),
		Interval:  types.Interval1m,
	}
	nextBucket, numShifts := bucket.findNextBucket(types.Time(bucket.EndTime.Time().Add(time.Second * 50)))
	assert.Equal(t, uint64(1), numShifts)
	assert.Equal(t, types.Time(bucket.StartTime.Time().Add(time.Minute)), nextBucket.StartTime)
	assert.Equal(t, types.Time(bucket.EndTime.Time().Add(time.Minute)), nextBucket.EndTime)

	// note that the start time is inclusive and the end time is exclusive
	nextBucket, numShifts = bucket.findNextBucket(types.Time(bucket.EndTime.Time().Add(time.Minute)))
	assert.Equal(t, uint64(2), numShifts)
	assert.Equal(t, types.Time(bucket.StartTime.Time().Add(time.Minute*2)), nextBucket.StartTime)
	assert.Equal(t, types.Time(bucket.EndTime.Time().Add(time.Minute*2)), nextBucket.EndTime)

	nextBucket, numShifts = bucket.findNextBucket(types.Time(bucket.EndTime.Time().Add(time.Minute).Add(10 * time.Second)))
	assert.Equal(t, uint64(2), numShifts)
	assert.Equal(t, types.Time(bucket.StartTime.Time().Add(time.Minute*2)), nextBucket.StartTime)
	assert.Equal(t, types.Time(bucket.EndTime.Time().Add(time.Minute*2)), nextBucket.EndTime)

	nextBucket, numShifts = bucket.findNextBucket(types.Time(bucket.EndTime.Time().Add(time.Minute * 2).Add(10 * time.Second)))
	assert.Equal(t, uint64(3), numShifts)
	assert.Equal(t, types.Time(bucket.StartTime.Time().Add(time.Minute*3)), nextBucket.StartTime)
	assert.Equal(t, types.Time(bucket.EndTime.Time().Add(time.Minute*3)), nextBucket.EndTime)
}
