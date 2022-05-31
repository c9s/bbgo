package batch

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func Test_TimeRangedQuery(t *testing.T) {
	startTime := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2021, time.January, 2, 0, 0, 0, 0, time.UTC)
	q := &AsyncTimeRangedBatchQuery{
		Type: time.Time{},
		T: func(obj interface{}) time.Time {
			return obj.(time.Time)
		},
		ID: func(obj interface{}) string {
			return strconv.FormatInt(obj.(time.Time).UnixMilli(), 10)
		},
		Q: func(startTime, endTime time.Time) (interface{}, error) {
			var cnt = 0
			var data []time.Time
			for startTime.Before(endTime) && cnt < 5 {
				d := startTime
				data = append(data, d)
				cnt++
				startTime = startTime.Add(time.Minute)
			}
			t.Logf("data: %v", data)
			return data, nil
		},
	}

	ch := make(chan time.Time, 100)

	// consumer
	go func() {
		for d := range ch {
			_ = d
		}
	}()
	errC := q.Query(context.Background(), ch, startTime, endTime)
	<-errC
}
