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
	// This function will be used for detecting duplicated objects.
	ID func(obj interface{}) string

	// Time is a function that returns the time of the object
	// This function will be used for sorting records
	Time func(obj interface{}) time.Time

	// Select is the select query builder for querying existing db records
	// The built SQL will be used for querying existing db records.
	// And then the ID function will be used for filtering duplicated object.
	Select squirrel.SelectBuilder

	// OnLoad is an optional field, which is called when the records are loaded from the database
	OnLoad func(objs interface{})

	// Filter is an optional field, which is used for filtering the remote records
	// Return true to keep the record,
	// Return false to filter the record.
	Filter func(obj interface{}) bool

	// BatchQuery is used for querying remote records.
	BatchQuery func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error)

	// Insert is an option field, which is used for customizing the record insert
	Insert func(obj interface{}) error

	// Insert is an option field, which is used for customizing the record batch insert
	BatchInsert func(obj interface{}) error

	BatchInsertBuffer int

	// LogInsert logs the insert record in INFO level
	LogInsert bool
}

func (sel SyncTask) execute(ctx context.Context, db *sqlx.DB, startTime time.Time, args ...time.Time) error {
	batchBufferRefVal := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(sel.Type)), 0, sel.BatchInsertBuffer)

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

	if err := sortRecordsAscending(sel, recordSliceRef); err != nil {
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

	defer func() {
		if sel.BatchInsert != nil && batchBufferRefVal.Len() > 0 {
			slice := batchBufferRefVal.Interface()
			if err := sel.BatchInsert(slice); err != nil {
				logrus.WithError(err).Errorf("batch insert error: %+v", slice)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			logrus.Warnf("context is cancelled, stop syncing")
			return ctx.Err()

		default:
			v, ok := dataCRef.Recv()
			if !ok {
				err := <-errC
				return err
			}

			obj := v.Interface()
			id := sel.ID(obj)
			if _, exists := ids[id]; exists {
				logrus.Debugf("object %s already exists, skipping", id)
				continue
			}

			tt := sel.Time(obj)
			if tt.Before(startTime) || tt.After(endTime) {
				logrus.Debugf("object %s time %s is outside of the time range", id, tt)
				continue
			}

			if sel.Filter != nil {
				if !sel.Filter(obj) {
					logrus.Debugf("object %s is filtered", id)
					continue
				}
			}

			ids[id] = struct{}{}
			if sel.BatchInsert != nil {
				if batchBufferRefVal.Len() > sel.BatchInsertBuffer-1 {
					if sel.LogInsert {
						logrus.Infof("batch inserting %d %T", batchBufferRefVal.Len(), obj)
					} else {
						logrus.Debugf("batch inserting %d %T", batchBufferRefVal.Len(), obj)
					}

					if err := sel.BatchInsert(batchBufferRefVal.Interface()); err != nil {
						return err
					}

					batchBufferRefVal = reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(sel.Type)), 0, sel.BatchInsertBuffer)
				}
				batchBufferRefVal = reflect.Append(batchBufferRefVal, v)
			} else {
				if sel.LogInsert {
					logrus.Infof("inserting %T: %+v", obj, obj)
				} else {
					logrus.Debugf("inserting %T: %+v", obj, obj)
				}
				if sel.Insert != nil {
					// for custom insert
					if err := sel.Insert(obj); err != nil {
						logrus.WithError(err).Errorf("can not insert record: %v", obj)
						return err
					}
				} else {
					if err := insertType(db, obj); err != nil {
						logrus.WithError(err).Errorf("can not insert record: %v", obj)
						return err
					}
				}
			}
		}
	}
}

func lastRecordTime(sel SyncTask, recordSlice reflect.Value, defaultTime time.Time) time.Time {
	since := defaultTime
	length := recordSlice.Len()
	if length > 0 {
		last := recordSlice.Index(length - 1)
		since = sel.Time(last.Interface())
	}

	return since
}

func sortRecordsAscending(sel SyncTask, recordSlice reflect.Value) error {
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
