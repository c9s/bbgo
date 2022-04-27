package binanceapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type TradeFee struct {
	Symbol          string `json:"symbol"`
	MakerCommission fixedpoint.Value `json:"makerCommission"`
	TakerCommission fixedpoint.Value  `json:"takerCommission"`
}

//go:generate requestgen -method GET -url "/sapi/v1/asset/tradeFee" -type GetTradeFeeRequest -responseType []TradeFee
type GetTradeFeeRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetTradeFeeRequest() *GetTradeFeeRequest {
	return &GetTradeFeeRequest{client: c}
}
