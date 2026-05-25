package binanceapi

import "github.com/c9s/requestgen"

type RestrictedAssetResponse struct {
	OpenLongRestrictedAsset    []string `json:"openLongRestrictedAsset"`
	MaxCollateralExceededAsset []string `json:"maxCollateralExceededAsset"`
}

//go:generate requestgen -method GET -url /sapi/v1/margin/restricted-asset -type RestrictedAssetRequest -responseType RestrictedAssetResponse
type RestrictedAssetRequest struct {
	client requestgen.APIClient
}

func (c *RestClient) NewRestrictedAssetRequest() *RestrictedAssetRequest {
	return &RestrictedAssetRequest{
		client: c,
	}
}
