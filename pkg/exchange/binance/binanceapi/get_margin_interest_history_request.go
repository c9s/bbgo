package binanceapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// interest type in response has 4 enums:
// PERIODIC interest charged per hour
// ON_BORROW first interest charged on borrow
// PERIODIC_CONVERTED interest charged per hour converted into BNB
// ON_BORROW_CONVERTED first interest charged on borrow converted into BNB
type InterestType string

const (
	InterestTypePeriodic          InterestType = "PERIODIC"
	InterestTypeOnBorrow          InterestType = "ON_BORROW"
	InterestTypePeriodicConverted InterestType = "PERIODIC_CONVERTED"
	InterestTypeOnBorrowConverted InterestType = "ON_BORROW_CONVERTED"
)

// MarginInterest is the user margin interest record
type MarginInterest struct {
	IsolatedSymbol      string                     `json:"isolatedSymbol"`
	Asset               string                     `json:"asset"`
	Interest            fixedpoint.Value           `json:"interest"`
	InterestAccuredTime types.MillisecondTimestamp `json:"interestAccuredTime"`
	InterestRate        fixedpoint.Value           `json:"interestRate"`
	Principal           fixedpoint.Value           `json:"principal"`
	Type                InterestType               `json:"type"`
}

//go:generate requestgen -method GET -url "/sapi/v1/margin/interestHistory" -type GetMarginInterestHistoryRequest -responseType .RowsResponse -responseDataField Rows -responseDataType []MarginInterest
type GetMarginInterestHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	asset          string     `param:"asset"`
	startTime      *time.Time `param:"startTime,milliseconds"`
	endTime        *time.Time `param:"endTime,milliseconds"`
	isolatedSymbol *string    `param:"isolatedSymbol"`
	archived       *bool      `param:"archived"`
	size           *int       `param:"size"`
	current        *int       `param:"current"`
}

func (c *RestClient) NewGetMarginInterestHistoryRequest() *GetMarginInterestHistoryRequest {
	return &GetMarginInterestHistoryRequest{client: c}
}
