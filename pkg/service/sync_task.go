package service

import (
	"context"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/util"
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

func (sel SyncTask) execute(
	ctx context.Context,
	db *sqlx.DB, startTime time.Time, endTimeArgs ...time.Time,
) error {
	logger := util.GetLoggerFromCtx(ctx)
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

	logger.Debugf("loaded %d %T records", recordSliceRef.Len(), sel.Type)

	ids := buildIdMap(sel, recordSliceRef)

	if err := sortRecordsAscending(sel, recordSliceRef); err != nil {
		return err
	}

	// NOTE: `detectLastSelfTrade` assumes that the last record is the most recent one
	useUpsert := false
	switch sel.Type.(type) {
	case types.Trade:
		// use upsert if the last record is a self-trade
		// or it will cause a duplicate key error
		useUpsert = detectLastestSelfTrade(ctx, db, sel, recordSliceRef)
	}

	if sel.OnLoad != nil {
		sel.OnLoad(recordSliceRef.Interface())
	}

	// default since time point
	startTime = lastRecordTime(sel, recordSliceRef, startTime)

	endTime := time.Now()
	if len(endTimeArgs) > 0 {
		endTime = endTimeArgs[0]
	}

	// asset "" means all assets
	dataC, errC := sel.BatchQuery(ctx, startTime, endTime)
	dataCRef := reflect.ValueOf(dataC)

	defer func() {
		if sel.BatchInsert != nil && batchBufferRefVal.Len() > 0 {
			slice := batchBufferRefVal.Interface()
			if err := sel.BatchInsert(slice); err != nil {
				logger.WithError(err).Errorf("batch insert error: %+v", slice)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			logger.Warnf("context is cancelled, stop syncing")
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
				logger.Debugf("object %s already exists, skipping", id)
				continue
			}

			tt := sel.Time(obj)
			if tt.Before(startTime) || tt.After(endTime) {
				logger.Debugf("object %s time %s is outside of the time range", id, tt)
				continue
			}

			if sel.Filter != nil {
				if !sel.Filter(obj) {
					logger.Debugf("object %s is filtered", id)
					continue
				}
			}

			ids[id] = struct{}{}
			if sel.BatchInsert != nil {
				if batchBufferRefVal.Len() > sel.BatchInsertBuffer-1 {
					if sel.LogInsert {
						logger.Infof("batch inserting %d %T", batchBufferRefVal.Len(), obj)
					} else {
						logger.Debugf("batch inserting %d %T", batchBufferRefVal.Len(), obj)
					}

					if err := sel.BatchInsert(batchBufferRefVal.Interface()); err != nil {
						return err
					}

					batchBufferRefVal = reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(sel.Type)), 0, sel.BatchInsertBuffer)
				}
				batchBufferRefVal = reflect.Append(batchBufferRefVal, v)
			} else {
				if sel.LogInsert {
					logger.Infof("inserting %T: %+v", obj, obj)
				} else {
					logger.Debugf("inserting %T: %+v", obj, obj)
				}
				var err error
				if sel.Insert != nil {
					// for custom insert
					err = sel.Insert(obj)
				} else {
					err = insertType(db, obj, useUpsert)
				}
				if isMysqlDuplicateError(err) {
					// ignore duplicate error
					logger.Warnf("duplicate entry for %T, skipped: %+v", obj, obj)
					continue
				} else if err != nil {
					logger.WithError(err).Errorf("can not insert record: %+v", obj)
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

func detectLastestSelfTrade(ctx context.Context, db *sqlx.DB, sel SyncTask, recordSliceRef reflect.Value) bool {
	// address the issue if the last record is a self-trade
	// detect self-trade by counting the number of trades with trade id as the lastest record
	// if the number of trades is 2, then it's a self-trade
	if recordSliceRef.Len() == 0 {
		logrus.Warnf("empty record slice")
		return false
	}
	// find the latest record by time
	latestTrade := recordSliceRef.Index(0).Interface().(types.Trade)
	latestTime := latestTrade.Time
	for i := 0; i < recordSliceRef.Len(); i++ {
		trade := recordSliceRef.Index(i).Interface().(types.Trade)
		if trade.Time.After(latestTime.Time()) {
			latestTime = trade.Time
			latestTrade = trade
		}
	}
	lastTradeId := strconv.FormatUint(latestTrade.ID, 10)
	query := squirrel.Select("*").
		From("trades").
		Where(squirrel.Eq{"id": lastTradeId})

	// Configure the correct placeholder format for the database driver
	switch db.DriverName() {
	case "postgres":
		query = query.PlaceholderFormat(squirrel.Dollar)
	}

	sql, args, err := query.ToSql()
	if err != nil {
		logrus.Warnf("can not build sql for self-trade records: %s", err)
		return false
	}
	rows, err := db.QueryxContext(ctx, sql, args...)
	if err != nil {
		logrus.Warnf("can not query self-trade records: %s", err)
		return false
	}
	defer rows.Close()

	records, err := scanRowsOfType(rows, sel.Type)
	if err != nil {
		return false
	}
	recordsRef := reflect.ValueOf(records)
	if recordsRef.Kind() == reflect.Ptr {
		recordsRef = recordsRef.Elem()
	}
	return recordsRef.Len() == 2
}

var dupKeyErrPattern = regexp.MustCompile("Duplicate entry")

func isMysqlDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	// check if it's a duplicate error:
	//    "Error 1062 (23000): Duplicate entry"
	if parentErr := errors.Unwrap(err); parentErr != nil {
		if mysqlErr, ok := parentErr.(*mysql.MySQLError); ok {
			return mysqlErr.Number == 1062
		}
	}

	return dupKeyErrPattern.MatchString(err.Error())
}
