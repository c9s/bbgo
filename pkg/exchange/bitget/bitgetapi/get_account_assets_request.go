package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type AccountAsset struct {
	CoinId    int64                      `json:"coinId"`
	CoinName  string                     `json:"coinName"`
	Available fixedpoint.Value           `json:"available"`
	Frozen    fixedpoint.Value           `json:"frozen"`
	Lock      fixedpoint.Value           `json:"lock"`
	UTime     types.MillisecondTimestamp `json:"uTime"`
}

//go:generate GetRequest -url "/api/spot/v1/account/assets" -type GetAccountAssetsRequest -responseDataType []AccountAsset
type GetAccountAssetsRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetAccountAssetsRequest() *GetAccountAssetsRequest {
	return &GetAccountAssetsRequest{client: c}
}
