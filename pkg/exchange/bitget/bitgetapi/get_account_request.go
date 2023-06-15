package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/types"
)

type Account struct {
	UserId      types.StrInt64 `json:"user_id"`
	InviterId   types.StrInt64 `json:"inviter_id"`
	Ips         string         `json:"ips"`
	Authorities []string       `json:"authorities"`
	ParentId    types.StrInt64 `json:"parentId"`
	Trader      bool           `json:"trader"`
}

//go:generate GetRequest -url "/api/spot/v1/account/getInfo" -type GetAccountRequest -responseDataType .Account
type GetAccountRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetAccountRequest() *GetAccountRequest {
	return &GetAccountRequest{client: c}
}
