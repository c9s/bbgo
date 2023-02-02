package binanceapi

import (
	"github.com/c9s/requestgen"
)

//type Trade = binance.TradeV3

//go:generate requestgen -method GET -url "/api/v3/historicalTrades" -type GetHistoricalTradesRequest -responseType []Trade
type GetHistoricalTradesRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string  `param:"symbol"`
	limit  *uint64 `param:"limit"`
	fromID *uint64 `param:"fromId"`
}

func (c *RestClient) NewGetHistoricalTradesRequest() *GetHistoricalTradesRequest {
	return &GetHistoricalTradesRequest{client: c}
}
