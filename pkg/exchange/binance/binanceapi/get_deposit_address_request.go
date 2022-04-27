package binanceapi

import (
	"github.com/c9s/requestgen"
)

type DepositAddress struct {
	Address string `json:"address"`
	Coin    string `json:"coin"`
	Tag     string `json:"tag"`
	Url     string `json:"url"`
}

//go:generate requestgen -method GET -url "/sapi/v1/capital/deposit/address" -type GetDepositAddressRequest -responseType .DepositAddress
type GetDepositAddressRequest struct {
	client requestgen.AuthenticatedAPIClient

	coin string `param:"coin"`

	network *string `param:"network"`
}

func (c *RestClient) NewGetDepositAddressRequest() *GetDepositAddressRequest {
	return &GetDepositAddressRequest{client: c}
}
