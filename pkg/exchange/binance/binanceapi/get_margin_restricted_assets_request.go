package binanceapi

import (
	"github.com/c9s/requestgen"
)

type GetMarginRestrictedAssetsResponse struct {
	OpenLongRestrictedAsset    []string `json:"openLongRestrictedAsset,omitzero"`
	MaxCollateralExceededAsset []string `json:"maxCollateralExceededAsset,omitzero"`
}

//go:generate requestgen -method GET -url "/sapi/v1/margin/restricted-asset" -type GetMarginRestrictedAssetsRequest -responseType .GetMarginRestrictedAssetsResponse
type GetMarginRestrictedAssetsRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetMarginRestrictedAssetsRequest() *GetMarginRestrictedAssetsRequest {
	return &GetMarginRestrictedAssetsRequest{client: c}
}
