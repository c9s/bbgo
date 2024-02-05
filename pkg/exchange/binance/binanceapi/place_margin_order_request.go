package binanceapi

import (
	"github.com/c9s/requestgen"
)

//go:generate requestgen -method POST -url "/sapi/v1/margin/borrow-repay" -type PlaceMarginOrderRequest -responseType .TransferResponse
type PlaceMarginOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	asset string `param:"asset"`

	// TRUE for Isolated Margin, FALSE for Cross Margin, Default FALSE
	isIsolated bool `param:"isIsolated"`

	// Only for Isolated margin
	symbol *string `param:"symbol"`

	amount string `param:"amount"`

	BorrowRepayType BorrowRepayType `param:"type"`
}

func (c *RestClient) NewPlaceMarginOrderRequest() *PlaceMarginOrderRequest {
	return &PlaceMarginOrderRequest{client: c}
}
