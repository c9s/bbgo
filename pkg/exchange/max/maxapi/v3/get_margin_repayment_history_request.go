package v3

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type RepaymentRecord struct {
	SN        string                     `json:"sn"`
	Currency  string                     `json:"currency"`
	Amount    fixedpoint.Value           `json:"amount"`
	Principal fixedpoint.Value           `json:"principal"`
	Interest  fixedpoint.Value           `json:"interest"`
	CreatedAt types.MillisecondTimestamp `json:"created_at"`
	State     string                     `json:"state"`
}

//go:generate GetRequest -url "/api/v3/wallet/m/repayments/:currency" -type GetMarginRepaymentHistoryRequest -responseType []RepaymentRecord
type GetMarginRepaymentHistoryRequest struct {
	client   requestgen.AuthenticatedAPIClient
	currency string `param:"currency,slug,required"`

	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
	limit     *int       `param:"limit"`
}
