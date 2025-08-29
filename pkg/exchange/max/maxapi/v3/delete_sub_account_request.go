package v3

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE
//go:generate DeleteRequest -url "/api/v3/sub_account" -type DeleteSubAccountRequest -responseType .SubAccount
type DeleteSubAccountRequest struct {
	client requestgen.AuthenticatedAPIClient

	sn string `param:"sn,required"`
}

func (s *SubAccountService) NewDeleteSubAccountRequest() *DeleteSubAccountRequest {
	return &DeleteSubAccountRequest{client: s.Client}
}
