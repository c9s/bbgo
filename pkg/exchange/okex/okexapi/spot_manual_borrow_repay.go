package okexapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type SpotBorrowRepayResponse struct {
	Currency string           `json:"ccy"`
	Side     string           `json:"side"`
	Amount   fixedpoint.Value `json:"amt"`
}

//go:generate PostRequest -url "/api/v5/account/set-leverage" -type SpotManualBorrowRepayRequest -responseDataType []SpotBorrowRepayResponse
type SpotManualBorrowRepayRequest struct {
	client requestgen.AuthenticatedAPIClient

	// side = borrow or repay
	side string `param:"side"`

	amount string `param:"amount"`
}

func (c *RestClient) NewSpotManualBorrowRepayRequest() *SpotManualBorrowRepayRequest {
	return &SpotManualBorrowRepayRequest{
		client: c,
	}
}
