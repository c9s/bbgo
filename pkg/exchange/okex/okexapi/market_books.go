package okexapi

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

type MarketBookResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		Asks  [][]fixedpoint.Value       `json:"asks"`
		Bids  [][]fixedpoint.Value       `json:"bids"`
		Ts    types.MillisecondTimestamp `json:"ts"`
		SeqID int64                      `json:"seqId"`
	} `json:"data"`
}

//go:generate requestgen -method GET -url "/api/v5/market/books" -type MarketBooksRequest -responseType MarketBookResponse
type MarketBooksRequest struct {
	client requestgen.APIClient

	instId string `param:"instId,required"`
	size   *int   `param:"sz"`
}

func (c *RestClient) NewMarketBooksRequest() *MarketBooksRequest {
	return &MarketBooksRequest{client: c}
}
