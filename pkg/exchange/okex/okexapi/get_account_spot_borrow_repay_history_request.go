package okexapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type MarginEventType string

const (
	MarginEventTypeAutoBorrow   MarginEventType = "auto_borrow"
	MarginEventTypeAutoRepay    MarginEventType = "auto_repay"
	MarginEventTypeManualBorrow MarginEventType = "manual_borrow"
	MarginEventTypeManualRepay  MarginEventType = "manual_repay"
)

type MarginHistoryEntry struct {
	AccumulatedBorrowAmount fixedpoint.Value `json:"accBorrowed"`

	Amount   fixedpoint.Value `json:"amt"`
	Currency string           `json:"ccy"`

	Ts   types.MillisecondTimestamp `json:"ts"`
	Type MarginEventType            `json:"type"`
}

//go:generate GetRequest -url "/api/v5/account/spot-borrow-repay-history" -type GetAccountSpotBorrowRepayHistoryRequest -responseDataType []MarginHistoryEntry
type GetAccountSpotBorrowRepayHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	eventType *MarginEventType `param:"type"`
	currency  *string          `param:"ccy"`
	after     *time.Time       `param:"after,milliseconds"`
	before    *time.Time       `param:"before,milliseconds"`
	limit     *uint64          `param:"limit" defaultValue:"100"`
}

func (c *RestClient) NewGetAccountSpotBorrowRepayHistoryRequest() *GetAccountSpotBorrowRepayHistoryRequest {
	return &GetAccountSpotBorrowRepayHistoryRequest{
		client: c,
	}
}
