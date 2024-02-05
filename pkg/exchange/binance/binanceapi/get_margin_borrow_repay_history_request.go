package binanceapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// one of PENDING (pending execution), CONFIRMED (successfully loaned), FAILED (execution failed, nothing happened to your account);
type MarginBorrowStatus string

const (
	BorrowRepayStatusPending   MarginBorrowStatus = "PENDING"
	BorrowRepayStatusConfirmed MarginBorrowStatus = "CONFIRMED"
	BorrowRepayStatusFailed    MarginBorrowStatus = "FAILED"
)

type BorrowRepayType string

const (
	BorrowRepayTypeBorrow BorrowRepayType = "BORROW"
	BorrowRepayTypeRepay  BorrowRepayType = "REPAY"
)

type MarginBorrowRepayRecord struct {
	IsolatedSymbol string                     `json:"isolatedSymbol"`
	Amount         fixedpoint.Value           `json:"amount"`
	Asset          string                     `json:"asset"`
	Interest       fixedpoint.Value           `json:"interest"`
	Principal      fixedpoint.Value           `json:"principal"`
	Status         MarginBorrowStatus         `json:"status"`
	Timestamp      types.MillisecondTimestamp `json:"timestamp"`
	TxId           uint64                     `json:"txId"`
}

// GetMarginBorrowRepayHistoryRequest
//
// txId or startTime must be sent. txId takes precedence.
// Response in descending order
// If isolatedSymbol is not sent, crossed margin data will be returned
// The max interval between startTime and endTime is 30 days.
// If startTime and endTime not sent, return records of the last 7 days by default
// Set archived to true to query data from 6 months ago
//
//go:generate requestgen -method GET -url "/sapi/v1/margin/borrow-repay" -type GetMarginBorrowRepayHistoryRequest -responseType .RowsResponse -responseDataField Rows -responseDataType []MarginBorrowRepayRecord
type GetMarginBorrowRepayHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	asset           string          `param:"asset"`
	startTime       *time.Time      `param:"startTime,milliseconds"`
	endTime         *time.Time      `param:"endTime,milliseconds"`
	isolatedSymbol  *string         `param:"isolatedSymbol"`
	archived        *bool           `param:"archived"`
	size            *int            `param:"size"`
	current         *int            `param:"current"`
	BorrowRepayType BorrowRepayType `param:"type"`
}

func (c *RestClient) NewGetMarginBorrowRepayHistoryRequest() *GetMarginBorrowRepayHistoryRequest {
	return &GetMarginBorrowRepayHistoryRequest{client: c}
}
