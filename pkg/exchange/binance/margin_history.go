package binance

import (
	"context"
	"fmt"
	"time"

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
	req.Size(100)

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

	var interests []types.MarginInterest
	for _, record := range records {
		interests = append(interests, toGlobalInterest(record))
	}

	return interests, err
}
