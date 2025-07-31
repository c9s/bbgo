package v3

import "github.com/c9s/requestgen"

//go:generate -command GetRequest requestgen -method GET

//go:generate GetRequest -url "/api/v3/info" -type GetUserInfoRequest -responseType UserInfo
type GetUserInfoRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (s *Client) NewGetUserInfoRequest() *GetUserInfoRequest {
	return &GetUserInfoRequest{client: s.Client}
}
