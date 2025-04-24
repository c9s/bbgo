package coinbase

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

type Ticker struct {
	TradeID           int              `json:"trade_id"`
	Ask               fixedpoint.Value `json:"ask"`
	Bid               fixedpoint.Value `json:"bid"`
	Volume            fixedpoint.Value `json:"volume"`
	Price             fixedpoint.Value `json:"price"`
	Size              fixedpoint.Value `json:"size"`
	Time              types.Time       `json:"time"`
	RfqVolume         string           `json:"rfq_volume"`
	ConversionsVolume string           `json:"conversions_volume"`
}

//go:generate requestgen -method GET -url /products/:product_id/ticker -rateLimiter 1+20/2s -type GetTickerRequest -responseType .Ticker
type GetTickerRequest struct {
	client requestgen.AuthenticatedAPIClient

	productID string `param:"product_id,slug,required"`
}

func (client *RestAPIClient) NewGetTickerRequest() *GetTickerRequest {
	return &GetTickerRequest{
		client: client,
	}
}
