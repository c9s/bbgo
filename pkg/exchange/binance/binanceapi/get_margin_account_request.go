package binanceapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type MarginAsset struct {
	Asset string `json:"asset"`

	Borrowed fixedpoint.Value `json:"borrowed"`
	Free     fixedpoint.Value `json:"free"`
	Interest fixedpoint.Value `json:"interest"`
	Locked   fixedpoint.Value `json:"locked"`
	NetAsset fixedpoint.Value `json:"netAsset"`
}

type CrossMarginAccount struct {
	Created       bool `json:"created"`
	BorrowEnabled bool `json:"borrowEnabled"`

	MarginLevel                fixedpoint.Value `json:"marginLevel"`
	CollateralMarginLevel      fixedpoint.Value `json:"collateralMarginLevel"`
	TotalAssetOfBtc            fixedpoint.Value `json:"totalAssetOfBtc"`
	TotalLiabilityOfBtc        fixedpoint.Value `json:"totalLiabilityOfBtc"`
	TotalNetAssetOfBtc         fixedpoint.Value `json:"totalNetAssetOfBtc"`
	TotalCollateralValueInUSDT fixedpoint.Value `json:"TotalCollateralValueInUSDT"`
	TotalOpenOrderLossInUSDT   fixedpoint.Value `json:"totalOpenOrderLossInUSDT"`

	TradeEnabled       bool   `json:"tradeEnabled"`
	TransferInEnabled  bool   `json:"transferInEnabled"`
	TransferOutEnabled bool   `json:"transferOutEnabled"`
	AccountType        string `json:"accountType"`

	UserAssets []MarginAsset `json:"userAssets"`
}

//go:generate requestgen -method GET -url "/sapi/v1/margin/account" -type GetMarginAccountRequest -responseType .CrossMarginAccount
type GetMarginAccountRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetMarginAccountRequest() *GetMarginAccountRequest {
	return &GetMarginAccountRequest{client: c}
}
