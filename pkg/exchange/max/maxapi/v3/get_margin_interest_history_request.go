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

type MarginInterestRecord struct {
	Currency     string                     `json:"currency"`
	Amount       fixedpoint.Value           `json:"amount"`
	InterestRate fixedpoint.Value           `json:"interest_rate"`
	CreatedAt    types.MillisecondTimestamp `json:"created_at"`
}

//go:generate GetRequest -url "/api/v3/wallet/m/interests" -type GetMarginInterestHistoryRequest -responseType []MarginInterestRecord
type GetMarginInterestHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency  string     `param:"currency,required"`
	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
	limit     *int       `param:"limit"`
}

func (s *Client) NewGetMarginInterestHistoryRequest(currency string) *GetMarginInterestHistoryRequest {
	return &GetMarginInterestHistoryRequest{client: s.Client, currency: currency}
}
