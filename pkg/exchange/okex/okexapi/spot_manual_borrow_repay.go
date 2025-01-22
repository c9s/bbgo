package okexapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type MarginSide string

const (
	MarginSideBorrow MarginSide = "borrow"
	MarginSideRepay  MarginSide = "repay"
)

type SpotBorrowRepayResponse struct {
	Currency string           `json:"ccy"`
	Side     MarginSide       `json:"side"`
	Amount   fixedpoint.Value `json:"amt"`
}

//go:generate PostRequest -url "/api/v5/account/spot-manual-borrow-repay" -type SpotManualBorrowRepayRequest -responseDataType []SpotBorrowRepayResponse -rateLimiter 1+20/2s
type SpotManualBorrowRepayRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency string `param:"ccy"`

	// side = borrow or repay
	side MarginSide `param:"side"`

	amount string `param:"amt"`
}

func (c *RestClient) NewSpotManualBorrowRepayRequest() *SpotManualBorrowRepayRequest {
	return &SpotManualBorrowRepayRequest{
		client: c,
	}
}
