package binanceapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/adshao/go-binance/v2"
)

type Trade = binance.TradeV3

//go:generate requestgen -method GET -url "/api/v3/myTrades" -type GetMyTradesRequest -responseType []Trade
type GetMyTradesRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol    string     `param:"symbol"`
	orderID   *uint64    `param:"orderId"`
	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
	fromID    *uint64    `param:"fromId"`
	limit     *uint64    `param:"limit"`
}

func (c *RestClient) NewGetMyTradesRequest() *GetMyTradesRequest {
	return &GetMyTradesRequest{client: c}
}
