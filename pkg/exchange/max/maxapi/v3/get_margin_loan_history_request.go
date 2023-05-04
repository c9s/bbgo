package v3

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func (s *Client) NewGetMarginLoanHistoryRequest() *GetMarginLoanHistoryRequest {
	return &GetMarginLoanHistoryRequest{client: s.Client}
}

type LoanRecord struct {
	SN           string                     `json:"sn"`
	Currency     string                     `json:"currency"`
	Amount       fixedpoint.Value           `json:"amount"`
	State        string                     `json:"state"`
	CreatedAt    types.MillisecondTimestamp `json:"created_at"`
	InterestRate fixedpoint.Value           `json:"interest_rate"`
}

//go:generate GetRequest -url "/api/v3/wallet/m/loans/:currency" -type GetMarginLoanHistoryRequest -responseType []LoanRecord
type GetMarginLoanHistoryRequest struct {
	client   requestgen.AuthenticatedAPIClient
	currency string `param:"currency,slug,required"`

	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
	limit     *int       `param:"limit"`
}
