package binance

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/types"
)

func (e *Exchange) QueryLoanHistory(ctx context.Context, asset string, startTime, endTime *time.Time) ([]types.MarginLoanRecord, error) {
	req := e.client2.NewGetMarginLoanHistoryRequest()
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

	var loans []types.MarginLoanRecord
	for _, record := range records {
		loans = append(loans, toGlobalLoan(record))
	}

	return loans, err
}

func toGlobalLoan(record binanceapi.MarginLoanRecord) types.MarginLoanRecord {
	return types.MarginLoanRecord{
		TransactionID:  uint64(record.TxId),
		Asset:          record.Asset,
		Principle:      record.Principal,
		Time:           types.Time(record.Timestamp),
		IsolatedSymbol: record.IsolatedSymbol,
	}
}

func (e *Exchange) QueryRepayHistory(ctx context.Context, asset string, startTime, endTime *time.Time) ([]types.MarginRepayRecord, error) {
	req := e.client2.NewGetMarginRepayHistoryRequest()
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

	var repays []types.MarginRepayRecord
	for _, record := range records {
		repays = append(repays, toGlobalRepay(record))
	}

	return repays, err
}

func toGlobalRepay(record binanceapi.MarginRepayRecord) types.MarginRepayRecord {
	return types.MarginRepayRecord{
		TransactionID:  record.TxId,
		Asset:          record.Asset,
		Principle:      record.Principal,
		Time:           types.Time(record.Timestamp),
		IsolatedSymbol: record.IsolatedSymbol,
	}
}

func (e *Exchange) QueryLiquidationHistory(ctx context.Context, startTime, endTime *time.Time) ([]types.MarginLiquidationRecord, error) {
	req := e.client2.NewGetMarginLiquidationHistoryRequest()

	if startTime != nil {
		req.StartTime(*startTime)
	}
	if endTime != nil {
		req.EndTime(*endTime)
	}

	if e.MarginSettings.IsIsolatedMargin {
		req.IsolatedSymbol(e.MarginSettings.IsolatedMarginSymbol)
	}

	_, err := req.Do(ctx)
	return nil, err
}

func (e *Exchange) QueryInterestHistory(ctx context.Context, asset string, startTime, endTime *time.Time) ([]types.MarginInterest, error) {
	req := e.client2.NewGetMarginInterestHistoryRequest()
	req.Asset(asset)

	if startTime != nil {
		req.StartTime(*startTime)
	}

	if endTime != nil {
		req.EndTime(*endTime)
	}

	if e.MarginSettings.IsIsolatedMargin {
		req.IsolatedSymbol(e.MarginSettings.IsolatedMarginSymbol)
	}

	_, err := req.Do(ctx)
	return nil, err
}
