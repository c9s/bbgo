package v3

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/types"
)

type SubAccount struct {
	SN        string                     `json:"sn"`
	Name      string                     `json:"name"`
	CreatedAt types.MillisecondTimestamp `json:"created_at"`
}

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE
//go:generate GetRequest -url "/api/v3/sub_accounts" -type GetSubAccountsRequest -responseType []SubAccount
type GetSubAccountsRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (s *SubAccountService) NewGetSubAccountsRequest() *GetSubAccountsRequest {
	return &GetSubAccountsRequest{client: s.Client}
}
