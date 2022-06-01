package service

import (
	"context"
	"reflect"
	"sort"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

// SyncTask defines the behaviors for syncing remote records
type SyncTask struct {
	Select     squirrel.SelectBuilder
	Type       interface{}
	BatchQuery func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error)

	// ID is a function that returns the unique identity of the object
	ID func(obj interface{}) string

	// Time is a function that returns the time of the object
	Time func(obj interface{}) time.Time
}

func (sel SyncTask) execute(ctx context.Context, db *sqlx.DB, startTime time.Time) error {

	// query from db
	recordSlice, err := selectAndScanType(ctx, db, sel.Select, sel.Type)
	if err != nil {
		return err
	}

	recordSliceRef := reflect.ValueOf(recordSlice)
	if recordSliceRef.Kind() == reflect.Ptr {
		recordSliceRef = recordSliceRef.Elem()
	}

	logrus.Debugf("loaded %d records", recordSliceRef.Len())

	ids := buildIdMap(sel, recordSliceRef)
	sortRecords(sel, recordSliceRef)

	// default since time point
	since := lastRecordTime(sel, recordSliceRef, startTime)

	// asset "" means all assets
	dataC, errC := sel.BatchQuery(ctx, since, time.Now())
	dataCRef := reflect.ValueOf(dataC)

	for {
		select {
		case <-ctx.Done():
			return nil

		case err := <-errC:
			return err

		default:
			v, ok := dataCRef.Recv()
			if !ok {
				err := <-errC
				return err
			}

			obj := v.Interface()
			id := sel.ID(obj)
			if _, exists := ids[id]; exists {
				continue
			}

			logrus.Debugf("inserting %T: %+v", obj, obj)
			if err := insertType(db, obj); err != nil {
				return err
			}
		}
	}
}

func lastRecordTime(sel SyncTask, recordSlice reflect.Value, defaultTime time.Time) time.Time {
	since := defaultTime
	length := recordSlice.Len()
	if length > 0 {
		since = sel.Time(recordSlice.Index(length - 1))
	}

	return since
}

func sortRecords(sel SyncTask, recordSlice reflect.Value) {
	// always sort
	sort.Slice(recordSlice.Interface(), func(i, j int) bool {
		a := sel.Time(recordSlice.Index(i).Interface())
		b := sel.Time(recordSlice.Index(j).Interface())
		return a.Before(b)
	})
}

func buildIdMap(sel SyncTask, recordSliceRef reflect.Value) map[string]struct{} {
	ids := map[string]struct{}{}
	for i := 0; i < recordSliceRef.Len(); i++ {
		entryRef := recordSliceRef.Index(i)
		id := sel.ID(entryRef.Interface())
		ids[id] = struct{}{}
	}

	return ids
}
