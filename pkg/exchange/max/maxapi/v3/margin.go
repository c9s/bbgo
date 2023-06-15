package v3

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import (
	"github.com/c9s/requestgen"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type MarginService struct {
	Client *maxapi.RestClient
}

func (s *Client) NewGetMarginBorrowingLimitsRequest() *GetMarginBorrowingLimitsRequest {
	return &GetMarginBorrowingLimitsRequest{client: s.Client}
}

type MarginBorrowingLimitMap map[string]fixedpoint.Value

//go:generate GetRequest -url "/api/v3/wallet/m/limits" -type GetMarginBorrowingLimitsRequest -responseType .MarginBorrowingLimitMap
type GetMarginBorrowingLimitsRequest struct {
	client requestgen.APIClient
}
