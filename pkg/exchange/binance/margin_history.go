package binance

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

func (e *Exchange) QueryLoanHistory(ctx context.Context, asset string, startTime, endTime *time.Time) ([]types.MarginLoan, error) {
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

	var loans []types.MarginLoan
	for _, record := range records {
		loans = append(loans, toGlobalLoan(record))
	}

	return loans, err
}

func (e *Exchange) QueryRepayHistory(ctx context.Context, asset string, startTime, endTime *time.Time) ([]types.MarginRepay, error) {
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

	var repays []types.MarginRepay
	for _, record := range records {
		repays = append(repays, toGlobalRepay(record))
	}

	return repays, err
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
