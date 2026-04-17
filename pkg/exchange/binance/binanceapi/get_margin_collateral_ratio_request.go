package binanceapi

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/requestgen"
)

type CollateralInfo struct {
	Collaterals []struct {
		MinUsdValue  fixedpoint.Value  `json:"minUsdValue"`
		MaxUsdValue  *fixedpoint.Value `json:"maxUsdValue,omitempty"`
		DiscountRate fixedpoint.Value  `json:"discountRate"`
	} `json:"collaterals"`
	AssetNames []string `json:"assetNames"`
}

//go:generate requestgen -method GET -url /sapi/v1/margin/crossMarginCollateralRatio -type GetMarginCollateralRatioRequest -responseType []CollateralInfo
type GetCrossMarginCollateralRatioRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetCrossMarginCollateralRatioRequest() *GetCrossMarginCollateralRatioRequest {
	return &GetCrossMarginCollateralRatioRequest{client: c}
}
