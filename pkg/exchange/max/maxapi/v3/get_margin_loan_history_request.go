package v3

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type LoanRecord struct {
	SN           string                     `json:"sn"`
	Currency     string                     `json:"currency"`
	Amount       fixedpoint.Value           `json:"amount"`
	State        string                     `json:"state"`
	CreatedAt    types.MillisecondTimestamp `json:"created_at"`
	InterestRate fixedpoint.Value           `json:"interest_rate"`
}

//go:generate GetRequest -url "/api/v3/wallet/m/loans" -type GetMarginLoanHistoryRequest -responseType []LoanRecord
type GetMarginLoanHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency string `param:"currency,required"`

	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
	limit     *int       `param:"limit"`
}

func (s *Client) NewGetMarginLoanHistoryRequest() *GetMarginLoanHistoryRequest {
	return &GetMarginLoanHistoryRequest{client: s.Client}
}
