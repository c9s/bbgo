package binanceapi

import (
	"github.com/c9s/requestgen"
)

type AccountType string

const (
	AccountTypeSpot           AccountType = "SPOT"
	AccountTypeIsolatedMargin AccountType = "ISOLATED_MARGIN"
)

//go:generate requestgen -method POST -url "/sapi/v1/margin/isolated/transfer" -type TransferIsolatedMarginAccountRequest -responseType .TransferResponse
type TransferIsolatedMarginAccountRequest struct {
	client requestgen.AuthenticatedAPIClient

	asset  string `param:"asset"`
	symbol string `param:"symbol"`

	// transFrom: "SPOT", "ISOLATED_MARGIN"
	transFrom AccountType `param:"transFrom"`

	// transTo: "SPOT", "ISOLATED_MARGIN"
	transTo AccountType `param:"transTo"`

	amount string `param:"amount"`
}

func (c *RestClient) NewTransferIsolatedMarginAccountRequest() *TransferIsolatedMarginAccountRequest {
	return &TransferIsolatedMarginAccountRequest{client: c}
}
