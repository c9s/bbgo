package binanceapi

import (
	"github.com/c9s/requestgen"
)

type TransferResponse struct {
	TranId int `json:"tranId"`
}

//go:generate requestgen -method POST -url "/sapi/v1/margin/transfer" -type TransferCrossMarginAccountRequest -responseType .TransferResponse
type TransferCrossMarginAccountRequest struct {
	client requestgen.AuthenticatedAPIClient

	asset string `param:"asset"`

	// transferType:
	// 1: transfer from main account to cross margin account
	// 2: transfer from cross margin account to main account
	transferType int `param:"type"`

	amount string `param:"amount"`
}

func (c *RestClient) NewTransferCrossMarginAccountRequest() *TransferCrossMarginAccountRequest {
	return &TransferCrossMarginAccountRequest{client: c}
}
