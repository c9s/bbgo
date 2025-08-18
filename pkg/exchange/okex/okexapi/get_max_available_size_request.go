package okexapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type MaxAvailableResponse struct {
	InstId        string           `json:"instId"`
	AvailableBuy  fixedpoint.Value `json:"availBuy"`
	AvailableSell fixedpoint.Value `json:"availSell"`
}

//go:generate GetRequest -url "/api/v5/account/max-avail-size" -type GetMaxAvailableSizeRequest -responseDataType []MaxAvailableResponse -rateLimiter 1+20/2s
type GetMaxAvailableSizeRequest struct {
	client requestgen.AuthenticatedAPIClient

	instrumentID string `param:"instId"`

	currency *string `param:"ccy"`

	tdMode TradeMode `param:"tdMode"`
}

func (c *RestClient) NewGetMaxAvailableSizeRequest() *GetMaxAvailableSizeRequest {
	return &GetMaxAvailableSizeRequest{
		client: c,
	}
}
