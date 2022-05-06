package binanceapi

import "github.com/c9s/requestgen"

type WalletType int

const (
	WalletTypeSpot    WalletType = 0
	WalletTypeFunding WalletType = 1
)

type WithdrawResponse struct {
	ID string `json:"id"`
}

//go:generate requestgen -method POST -url "/sapi/v1/capital/withdraw/apply" -type WithdrawRequest -responseType .WithdrawResponse
type WithdrawRequest struct {
	client  requestgen.AuthenticatedAPIClient
	coin    string  `param:"coin"`
	network *string `param:"network"`

	address    string  `param:"address"`
	addressTag *string `param:"addressTag"`

	// amount is a decimal in string format
	amount string `param:"amount"`

	withdrawOrderId *string `param:"withdrawOrderId"`

	transactionFeeFlag *bool `param:"transactionFeeFlag"`

	// name is the address name
	name *string `param:"name"`

	// The wallet type for withdraw: 0-spot wallet ，1-funding wallet.Default spot wallet
	walletType *WalletType `param:"walletType"`
}

func (c *RestClient) NewWithdrawRequest() *WithdrawRequest {
	return &WithdrawRequest{client: c}
}
