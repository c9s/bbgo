package v3

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

func (s *MarginService) NewGetMarginADRatioRequest() *GetMarginADRatioRequest {
	return &GetMarginADRatioRequest{client: s.Client}
}

type ADRatio struct {
	AdRatio     fixedpoint.Value `json:"ad_ratio"`
	AssetInUsdt fixedpoint.Value `json:"asset_in_usdt"`
	DebtInUsdt  fixedpoint.Value `json:"debt_in_usdt"`
}

//go:generate GetRequest -url "/api/v3/wallet/m/ad_ratio" -type GetMarginADRatioRequest -responseType .ADRatio
type GetMarginADRatioRequest struct {
	client requestgen.AuthenticatedAPIClient
}
