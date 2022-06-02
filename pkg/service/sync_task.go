package service

import (
	"context"
	"reflect"
	"sort"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// SyncTask defines the behaviors for syncing remote records
type SyncTask struct {
	// Type is the element type of this sync task
	// Since it will create a []Type slice from this type, you should not set pointer to this field
	Type interface{}

	// ID is a function that returns the unique identity of the object
	ID func(obj interface{}) string

	// Time is a function that returns the time of the object
	Time func(obj interface{}) time.Time

	// Select is the select query builder for querying db records
	Select squirrel.SelectBuilder

	// OnLoad is an optional field, which is called when the records are loaded from the database
	OnLoad func(objs interface{})

	// Filter is an optional field, which is used for filtering the remote records
	Filter func(obj interface{}) bool

	// Insert is an option field, which is used for customizing the record insert
	Insert func(obj interface{}) error

	// BatchQuery is used for querying remote records.
	BatchQuery func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error)
}

func (sel SyncTask) execute(ctx context.Context, db *sqlx.DB, startTime time.Time, args ...time.Time) error {
	// query from db
	recordSlice, err := selectAndScanType(ctx, db, sel.Select, sel.Type)
	if err != nil {
		return err
	}

	recordSliceRef := reflect.ValueOf(recordSlice)
	if recordSliceRef.Kind() == reflect.Ptr {
		recordSliceRef = recordSliceRef.Elem()
	}

	logrus.Debugf("loaded %d %T records", recordSliceRef.Len(), sel.Type)

	ids := buildIdMap(sel, recordSliceRef)

	if err := sortRecords(sel, recordSliceRef); err != nil {
		return err
	}

	if sel.OnLoad != nil {
		sel.OnLoad(recordSliceRef.Interface())
	}

	// default since time point
	startTime = lastRecordTime(sel, recordSliceRef, startTime)

	endTime := time.Now()
	if len(args) > 0 {
		endTime = args[0]
	}

	// asset "" means all assets
	dataC, errC := sel.BatchQuery(ctx, startTime, endTime)
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

			if sel.Filter != nil {
				if !sel.Filter(obj) {
					continue
				}
			}

			logrus.Infof("inserting %T: %+v", obj, obj)

			if sel.Insert != nil {
				// for custom insert
				if err := sel.Insert(obj); err != nil {
					return err
				}
			} else {
				if err := insertType(db, obj); err != nil {
					return err
				}
			}
		}
	}
}

func lastRecordTime(sel SyncTask, recordSlice reflect.Value, defaultTime time.Time) time.Time {
	since := defaultTime
	length := recordSlice.Len()
	if length > 0 {
		since = sel.Time(recordSlice.Index(length - 1).Interface())
	}

	return since
}

func sortRecords(sel SyncTask, recordSlice reflect.Value) error {
	if sel.Time == nil {
		return errors.New("time field is not set, can not sort records")
	}

	// always sort
	sort.Slice(recordSlice.Interface(), func(i, j int) bool {
		a := sel.Time(recordSlice.Index(i).Interface())
		b := sel.Time(recordSlice.Index(j).Interface())
		return a.Before(b)
	})
	return nil
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


