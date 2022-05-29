package binanceapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// RepayStatus one of PENDING (pending execution), CONFIRMED (successfully loaned), FAILED (execution failed, nothing happened to your account);
type RepayStatus string

const (
	RepayStatusPending   LoanStatus = "PENDING"
	RepayStatusConfirmed LoanStatus = "CONFIRMED"
	RepayStatusFailed    LoanStatus = "FAILED"
)

type MarginRepayRecord struct {
	IsolatedSymbol string                     `json:"isolatedSymbol"`
	Amount         fixedpoint.Value           `json:"amount"`
	Asset          string                     `json:"asset"`
	Interest       fixedpoint.Value           `json:"interest"`
	Principal      fixedpoint.Value           `json:"principal"`
	Status         string                     `json:"status"`
	Timestamp      types.MillisecondTimestamp `json:"timestamp"`
	TxId           uint64                     `json:"txId"`
}

//go:generate requestgen -method GET -url "/sapi/v1/margin/repay" -type GetMarginRepayHistoryRequest -responseType .RowsResponse -responseDataField Rows -responseDataType []MarginRepayRecord
type GetMarginRepayHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	asset          string     `param:"asset"`
	startTime      *time.Time `param:"startTime,milliseconds"`
	endTime        *time.Time `param:"endTime,milliseconds"`
	isolatedSymbol *string    `param:"isolatedSymbol"`
	archived       *bool      `param:"archived"`
	size           *int       `param:"size"`
	current        *int       `param:"current"`
}

func (c *RestClient) NewGetMarginRepayHistoryRequest() *GetMarginRepayHistoryRequest {
	return &GetMarginRepayHistoryRequest{client: c}
}
