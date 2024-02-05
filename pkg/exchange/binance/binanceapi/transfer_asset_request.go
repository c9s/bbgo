package binanceapi

import (
	"github.com/c9s/requestgen"
)

type TransferResponse struct {
	TranId int `json:"tranId"`
}

type TransferAssetType string

const (
	TransferAssetTypeMainToMargin         TransferAssetType = "MAIN_MARGIN"
	TransferAssetTypeMarginToMain         TransferAssetType = "MARGIN_MAIN"
	TransferAssetTypeMainToIsolatedMargin TransferAssetType = "MAIN_ISOLATED_MARGIN"
	TransferAssetTypeIsolatedMarginToMain TransferAssetType = "ISOLATED_MARGIN_MAIN"
)

//go:generate requestgen -method POST -url "/sapi/v1/asset/transfer" -type TransferAssetRequest -responseType .TransferResponse
type TransferAssetRequest struct {
	client requestgen.AuthenticatedAPIClient

	asset string `param:"asset"`

	transferType TransferAssetType `param:"type"`

	amount string `param:"amount"`

	fromSymbol *string `param:"fromSymbol"`
	toSymbol   *string `param:"toSymbol"`
}

func (c *RestClient) NewTransferAssetRequest() *TransferAssetRequest {
	return &TransferAssetRequest{client: c}
}
