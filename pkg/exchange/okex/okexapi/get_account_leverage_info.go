package okexapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type LeverageInfo struct {
	Ccy     string           `json:"ccy"`
	InstId  string           `json:"instId"`
	MgnMode MarginMode       `json:"mgnMode"`
	PosSide string           `json:"posSide"`
	Lever   fixedpoint.Value `json:"lever"`
}

//go:generate GetRequest -url "/api/v5/account/leverage-info" -type GetAccountLeverageInfoRequest -responseDataType []LeverageInfo
type GetAccountLeverageInfoRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency *string `param:"ccy"`

	marginMode MarginMode `param:"mgnMode"`
}

func (c *RestClient) NewGetAccountLeverageInfoRequest() *GetAccountLeverageInfoRequest {
	return &GetAccountLeverageInfoRequest{
		client: c,
	}
}
