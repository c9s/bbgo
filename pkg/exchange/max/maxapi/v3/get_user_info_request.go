package v3

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/exchange/max/maxapi"
)

type UserInfo maxapi.UserInfo

//go:generate -command GetRequest requestgen -method GET

//go:generate GetRequest -url "/api/v3/info" -type GetUserInfoRequest -responseType .UserInfo
type GetUserInfoRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *Client) NewGetUserInfoRequest() *GetUserInfoRequest {
	return &GetUserInfoRequest{client: c.RestClient}
}
