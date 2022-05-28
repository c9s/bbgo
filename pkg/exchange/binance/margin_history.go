package binance

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

func (e *Exchange) QueryLoanHistory(ctx context.Context, asset string, startTime, endTime *time.Time) ([]types.MarginLoanRecord, error) {
	req := e.client2.NewGetMarginLoanHistoryRequest()
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

	loans, err := req.Do(ctx)
	_ = loans
	return nil, err
}

func (e *Exchange) QueryRepayHistory(ctx context.Context, asset string, startTime, endTime *time.Time) ([]types.MarginRepayRecord, error) {
	req := e.client2.NewGetMarginRepayHistoryRequest()
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
