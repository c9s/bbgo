package v3

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type SubAccountTransfer struct {
	Sn       string           `json:"sn"`
	Currency string           `json:"currency"`
	Amount   fixedpoint.Value `json:"amount"`
	State    string           `json:"state"`

	CreatedAt types.MillisecondTimestamp `json:"created_at"`

	FromSubAccount struct {
		SN string `json:"sn"`
	} `json:"from_sub_account"`

	ToSubAccount struct {
		SN string `json:"sn"`
	} `json:"to_sub_account"`
}

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE
//go:generate PostRequest -url "/api/v3/sub_account/transfer" -type SubmitSubAccountTransferRequest -responseType .SubAccountTransfer
type SubmitSubAccountTransferRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency string `param:"currency,required"`

	amount string `param:"amount,required"`

	toSN *string `param:"to_sn"`

	toMain *bool `param:"to_main"`
}

func (s *SubAccountService) NewSubmitSubAccountTransferRequest() *SubmitSubAccountTransferRequest {
	return &SubmitSubAccountTransferRequest{client: s.Client}
}
