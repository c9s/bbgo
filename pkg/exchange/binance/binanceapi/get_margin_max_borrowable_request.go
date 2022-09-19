package binanceapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// MarginMaxBorrowable is the user margin interest record
type MarginMaxBorrowable struct {
	Amount      fixedpoint.Value `json:"amount"`
	BorrowLimit fixedpoint.Value `json:"borrowLimit"`
}

//go:generate requestgen -method GET -url "/sapi/v1/margin/maxBorrowable" -type GetMarginMaxBorrowableRequest -responseType .MarginMaxBorrowable
type GetMarginMaxBorrowableRequest struct {
	client requestgen.AuthenticatedAPIClient

	asset          string  `param:"asset"`
	isolatedSymbol *string `param:"isolatedSymbol"`
}

func (c *RestClient) NewGetMarginMaxBorrowableRequest() *GetMarginMaxBorrowableRequest {
	return &GetMarginMaxBorrowableRequest{client: c}
}
