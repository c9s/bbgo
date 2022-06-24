package batch

import (
	"context"
	"reflect"
	"sort"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/util"
)

var log = logrus.WithField("component", "batch")

type AsyncTimeRangedBatchQuery struct {
	// Type is the object type of the result
	Type interface{}

	// Limiter is the rate limiter for each query
	Limiter *rate.Limiter

	// Q is the remote query function
	Q func(startTime, endTime time.Time) (interface{}, error)

	// T function returns time of an object
	T func(obj interface{}) time.Time

	// ID returns the ID of the object
	ID func(obj interface{}) string

	// JumpIfEmpty jump the startTime + duration when the result is empty
	JumpIfEmpty time.Duration
}

func (q *AsyncTimeRangedBatchQuery) Query(ctx context.Context, ch interface{}, since, until time.Time) chan error {
	errC := make(chan error, 1)
	cRef := reflect.ValueOf(ch)
	// cRef := reflect.MakeChan(reflect.TypeOf(q.Type), 100)
	startTime := since
	endTime := until

	go func() {
		defer cRef.Close()
		defer close(errC)

		idMap := make(map[string]struct{}, 100)
		for startTime.Before(endTime) {
			if q.Limiter != nil {
				if err := q.Limiter.Wait(ctx); err != nil {
					errC <- err
					return
				}
			}

			log.Debugf("batch querying %T: %v <=> %v", q.Type, startTime, endTime)

			queryProfiler := util.StartTimeProfile("remoteQuery")

			sliceInf, err := q.Q(startTime, endTime)
			if err != nil {
				errC <- err
				return
			}

			listRef := reflect.ValueOf(sliceInf)
			listLen := listRef.Len()
			log.Debugf("batch querying %T: %d remote records", q.Type, listLen)

			queryProfiler.StopAndLog(log.Debugf)

			if listLen == 0 {
				if q.JumpIfEmpty > 0 {
					startTime = startTime.Add(q.JumpIfEmpty)

					log.Debugf("batch querying %T: empty records jump to %s", q.Type, startTime)
					continue
				}

				log.Debugf("batch querying %T: empty records, query is completed", q.Type)
				return
			}

			// sort by time
			sort.Slice(listRef.Interface(), func(i, j int) bool {
				a := listRef.Index(i)
				b := listRef.Index(j)
				tA := q.T(a.Interface())
				tB := q.T(b.Interface())
				return tA.Before(tB)
			})

			sentAny := false
			for i := 0; i < listLen; i++ {
				item := listRef.Index(i)
				entryTime := q.T(item.Interface())
				if entryTime.Before(since) || entryTime.After(until) {
					continue
				}

				obj := item.Interface()
				id := q.ID(obj)
				if _, exists := idMap[id]; exists {
					log.Debugf("batch querying %T: ignore duplicated record, id = %s", q.Type, id)
					continue
				}

				idMap[id] = struct{}{}

				cRef.Send(item)
				sentAny = true
				startTime = entryTime
			}

			if !sentAny {
				log.Debugf("batch querying %T: %d/%d records are not sent", q.Type, listLen, listLen)
				return
			}
		}
	}()

	return errC
}
