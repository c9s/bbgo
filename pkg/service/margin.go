package service

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
)

// SyncSelect defines the behaviors for syncing remote records
type SyncSelect struct {
	Select     sq.SelectBuilder
	Type       interface{}
	BatchQuery func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error)

	// ID is a function that returns the unique identity of the object
	ID func(obj interface{}) string

	// Time is a function that returns the time of the object
	Time func(obj interface{}) time.Time
}

type MarginService struct {
	DB *sqlx.DB
}

func (s *MarginService) Sync(ctx context.Context, ex types.Exchange, asset string, startTime time.Time) error {
	api, ok := ex.(types.MarginHistory)
	if !ok {
		return ErrNotImplemented
	}

	marginExchange, ok := ex.(types.MarginExchange)
	if !ok {
		return fmt.Errorf("%T does not implement margin service", ex)
	}

	marginSettings := marginExchange.GetMarginSettings()
	if !marginSettings.IsMargin {
		return fmt.Errorf("exchange instance %s is not using margin", ex.Name())
	}

	sels := []SyncSelect{
		{
			Select: SelectLastMarginLoans(ex.Name(), 100),
			Type:   types.MarginLoan{},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.MarginLoanBatchQuery{
					MarginHistory: api,
				}
				return query.Query(ctx, asset, startTime, endTime)
			},
			ID: func(obj interface{}) string {
				return strconv.FormatUint(obj.(types.MarginLoan).TransactionID, 10)
			},
		},
		{
			Select: SelectLastMarginRepays(ex.Name(), 100),
			Type:   types.MarginRepay{},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.MarginRepayBatchQuery{
					MarginHistory: api,
				}
				return query.Query(ctx, asset, startTime, endTime)
			},
			ID: func(obj interface{}) string {
				return strconv.FormatUint(obj.(types.MarginRepay).TransactionID, 10)
			},
		},
		{
			Select: SelectLastMarginInterests(ex.Name(), 100),
			Type:   types.MarginInterest{},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.MarginInterestBatchQuery{
					MarginHistory: api,
				}
				return query.Query(ctx, asset, startTime, endTime)
			},
			ID: func(obj interface{}) string {
				m := obj.(types.MarginInterest)
				return m.Asset + m.IsolatedSymbol + strconv.FormatInt(m.Time.UnixMilli(), 10)
			},
		},
	}

NextQuery:
	for _, sel := range sels {
		// query from db
		recordSlice, err := s.executeDbQuery(ctx, sel.Select, sel.Type)
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
					if err != nil {
						return err
					}

					// closed chan, skip to next query
					continue NextQuery
				}

				obj := v.Interface()
				id := sel.ID(obj)
				if _, exists := ids[id]; exists {
					continue
				}

				logrus.Debugf("inserting %T: %+v", obj, obj)
				if err := s.Insert(obj); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (s *MarginService) executeDbQuery(ctx context.Context, sel sq.SelectBuilder, tpe interface{}) (interface{}, error) {
	sql, args, err := sel.ToSql()

	if err != nil {
		return nil, err
	}

	rows, err := s.DB.QueryxContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	return s.scanRows(rows, tpe)
}

func (s *MarginService) scanRows(rows *sqlx.Rows, tpe interface{}) (interface{}, error) {
	refType := reflect.TypeOf(tpe)

	if refType.Kind() == reflect.Ptr {
		refType = refType.Elem()
	}

	sliceRef := reflect.New(reflect.SliceOf(refType))
	for rows.Next() {
		var recordRef = reflect.New(refType)
		var record = recordRef.Interface()
		if err := rows.StructScan(&record); err != nil {
			return sliceRef.Interface(), err
		}

		sliceRef = reflect.Append(sliceRef, recordRef)
	}

	return sliceRef.Interface(), rows.Err()
}

func (s *MarginService) Insert(record interface{}) error {
	sql := dbCache.InsertSqlOf(record)
	_, err := s.DB.NamedExec(sql, record)
	return err
}

func SelectLastMarginLoans(ex types.ExchangeName, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("margin_loans").
		Where(sq.Eq{"exchange": ex}).
		OrderBy("time").
		Limit(limit)
}

func SelectLastMarginRepays(ex types.ExchangeName, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("margin_repays").
		Where(sq.Eq{"exchange": ex}).
		OrderBy("time").
		Limit(limit)
}

func SelectLastMarginInterests(ex types.ExchangeName, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("margin_interests").
		Where(sq.Eq{"exchange": ex}).
		OrderBy("time").
		Limit(limit)
}

func SelectLastMarginLiquidations(ex types.ExchangeName, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("margin_liquidations").
		Where(sq.Eq{"exchange": ex}).
		OrderBy("time").
		Limit(limit)
}

func lastRecordTime(sel SyncSelect, recordSlice reflect.Value, defaultTime time.Time) time.Time {
	since := defaultTime
	length := recordSlice.Len()
	if length > 0 {
		since = sel.Time(recordSlice.Index(length - 1))
	}

	return since
}

func sortRecords(sel SyncSelect, recordSlice reflect.Value) {
	// always sort
	sort.Slice(recordSlice.Interface(), func(i, j int) bool {
		a := sel.Time(recordSlice.Index(i).Interface())
		b := sel.Time(recordSlice.Index(j).Interface())
		return a.Before(b)
	})
}

func buildIdMap(sel SyncSelect, recordSliceRef reflect.Value) map[string]struct{} {
	ids := map[string]struct{}{}
	for i := 0; i < recordSliceRef.Len(); i++ {
		entryRef := recordSliceRef.Index(i)
		id := sel.ID(entryRef.Interface())
		ids[id] = struct{}{}
	}

	return ids
}
