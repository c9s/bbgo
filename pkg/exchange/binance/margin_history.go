package binance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/types"
)

type BorrowRepayType interface {
	types.MarginLoan | types.MarginRepay
}

func queryBorrowRepayHistory[T BorrowRepayType](e *Exchange, ctx context.Context, asset string, startTime, endTime *time.Time) ([]T, error) {
	req := e.client2.NewGetMarginBorrowRepayHistoryRequest()
	req.Asset(asset)
	req.Size(100)

	switch v := any(T{}); v.(type) {
	case types.MarginLoan:
		req.SetBorrowRepayType(binanceapi.BorrowRepayTypeBorrow)
	case types.MarginRepay:
		req.SetBorrowRepayType(binanceapi.BorrowRepayTypeRepay)
	default:
		return nil, fmt.Errorf("T is other type")
	}

	if startTime != nil {
		req.StartTime(*startTime)

		// 6 months
		if time.Since(*startTime) > time.Hour*24*30*6 {
			req.Archived(true)
		}
	}

	if startTime != nil && endTime != nil {
		duration := endTime.Sub(*startTime)
		if duration > time.Hour*24*30 {
			t := startTime.Add(time.Hour * 24 * 30)
			endTime = &t
		}
	}

	if endTime != nil {
		req.EndTime(*endTime)
	}

	if e.MarginSettings.IsIsolatedMargin {
		req.IsolatedSymbol(e.MarginSettings.IsolatedMarginSymbol)
	}

	records, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var borrowRepay []T
	for _, record := range records {
		borrowRepay = append(borrowRepay, T{
			Exchange:       types.ExchangeBinance,
			TransactionID:  record.TxId,
			Asset:          record.Asset,
			Principle:      record.Principal,
			Time:           types.Time(record.Timestamp),
			IsolatedSymbol: record.IsolatedSymbol,
		})
	}

	return borrowRepay, nil
}

func (e *Exchange) QueryLoanHistory(ctx context.Context, asset string, startTime, endTime *time.Time) ([]types.MarginLoan, error) {
	return queryBorrowRepayHistory[types.MarginLoan](e, ctx, asset, startTime, endTime)
}

func (e *Exchange) QueryRepayHistory(ctx context.Context, asset string, startTime, endTime *time.Time) ([]types.MarginRepay, error) {
	return queryBorrowRepayHistory[types.MarginRepay](e, ctx, asset, startTime, endTime)
}

func (e *Exchange) QueryLiquidationHistory(ctx context.Context, startTime, endTime *time.Time) ([]types.MarginLiquidation, error) {
	req := e.client2.NewGetMarginLiquidationHistoryRequest()
	req.Size(100)

	if startTime != nil {
		req.StartTime(*startTime)
	}

	if startTime != nil && endTime != nil {
		duration := endTime.Sub(*startTime)
		if duration > time.Hour*24*30 {
			t := startTime.Add(time.Hour * 24 * 30)
			endTime = &t
		}
	}

	if endTime != nil {
		req.EndTime(*endTime)
	}

	if e.MarginSettings.IsIsolatedMargin {
		req.IsolatedSymbol(e.MarginSettings.IsolatedMarginSymbol)
	}

	records, err := req.Do(ctx)
	var liquidations []types.MarginLiquidation
	for _, record := range records {
		liquidations = append(liquidations, toGlobalLiquidation(record))
	}

	return liquidations, err
}

func (e *Exchange) QueryInterestHistory(ctx context.Context, asset string, startTime, endTime *time.Time) ([]types.MarginInterest, error) {
	req := e.client2.NewGetMarginInterestHistoryRequest()
	req.Asset(asset)
	req.Size(interestHistoryPageSize)

	if startTime != nil {
		req.StartTime(*startTime)

		// 6 months
		if time.Since(*startTime) > time.Hour*24*30*6 {
			req.Archived(true)
		}
	}

	if startTime != nil && endTime != nil {
		duration := endTime.Sub(*startTime)
		if duration > time.Hour*24*30 {
			t := startTime.Add(time.Hour * 24 * 30)
			endTime = &t
		}
	}

	if e.MarginSettings.IsIsolatedMargin {
		req.IsolatedSymbol(e.MarginSettings.IsolatedMarginSymbol)
	}

	records, err := queryInterestWindow(ctx, startTime, endTime,
		func(ctx context.Context, end *time.Time, page int) ([]binanceapi.MarginInterest, error) {
			if end != nil {
				req.EndTime(*end)
			}
			req.Current(page)
			return req.Do(ctx)
		})
	if err != nil {
		return nil, err
	}

	var interests []types.MarginInterest
	for _, record := range records {
		interests = append(interests, toGlobalInterest(record))
	}

	return interests, nil
}

const (
	// interestHistoryPageSize is the page size sent to Binance and the short-page test of queryInterestPages
	interestHistoryPageSize = 100

	// interestHistoryMaxPages caps one query at 10000 rows. A 30-day window has
	// 720 hourly charges per asset plus one ON_BORROW row per borrow.
	interestHistoryMaxPages = 100

	// interestHistoryMinWindow is the narrowest window queryInterestWindow halves down to
	interestHistoryMinWindow = time.Hour
)

// errInterestPageCap reports a window with more than interestHistoryMaxPages pages
var errInterestPageCap = errors.New("margin interest history window has too many pages")

// queryInterestWindow fetches the rows of [start, end]. A window over the page
// cap is halved, [start, start+(end-start)/2], until its rows fit, so the batch
// query moves forward from the narrowed window instead of failing on the same
// window on every sync. A window that cannot be narrowed is a permanent error.
func queryInterestWindow(
	ctx context.Context, start, end *time.Time,
	fetch func(ctx context.Context, end *time.Time, page int) ([]binanceapi.MarginInterest, error),
) ([]binanceapi.MarginInterest, error) {
	for {
		rows, err := queryInterestPages(ctx, func(ctx context.Context, page int) ([]binanceapi.MarginInterest, error) {
			return fetch(ctx, end, page)
		})
		if !errors.Is(err, errInterestPageCap) {
			return rows, err
		}

		if start == nil || end == nil || end.Sub(*start) <= interestHistoryMinWindow {
			return nil, backoff.Permanent(fmt.Errorf("%w: more than %d pages of %d rows", err, interestHistoryMaxPages, interestHistoryPageSize))
		}

		half := start.Add(end.Sub(*start) / 2)
		end = &half
	}
}

// queryInterestPages fetches pages 1, 2, ... until a page has fewer than
// interestHistoryPageSize rows. The batch query moves its start time to the
// newest row it received, so a single page would skip the older rows of the
// window when Binance returns the newest rows first.
//
// Hitting the page cap returns errInterestPageCap and no rows: returning them
// would let the batch query move past the rows that were never fetched.
// queryInterestWindow narrows the window instead.
func queryInterestPages(
	ctx context.Context, fetch func(ctx context.Context, page int) ([]binanceapi.MarginInterest, error),
) ([]binanceapi.MarginInterest, error) {
	var rows []binanceapi.MarginInterest
	for page := 1; page <= interestHistoryMaxPages; page++ {
		records, err := fetch(ctx, page)
		if err != nil {
			return nil, err
		}

		rows = append(rows, records...)
		if len(records) < interestHistoryPageSize {
			return rows, nil
		}
	}

	return nil, errInterestPageCap
}
