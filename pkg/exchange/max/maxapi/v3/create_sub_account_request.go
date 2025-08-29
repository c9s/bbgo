package v3

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE
//go:generate PostRequest -url "/api/v3/sub_accounts" -type CreateSubAccountRequest -responseType .SubAccount
type CreateSubAccountRequest struct {
	client requestgen.AuthenticatedAPIClient

	name string `param:"name,required"`
}

func (s *SubAccountService) NewCreateSubAccountRequest() *CreateSubAccountRequest {
	return &CreateSubAccountRequest{client: s.Client}
}
