package binanceapi

import (
	"time"

	"github.com/c9s/requestgen"
)

//go:generate requestgen -method GET -url "/sapi/v1/margin/myTrades" -type GetMarginTradesRequest -responseType []Trade
type GetMarginTradesRequest struct {
	client requestgen.AuthenticatedAPIClient

	isIsolated bool       `param:"isIsolated"`
	symbol     string     `param:"symbol"`
	orderID    *uint64    `param:"orderId"`
	startTime  *time.Time `param:"startTime,milliseconds"`
	endTime    *time.Time `param:"endTime,milliseconds"`
	fromID     *uint64    `param:"fromId"`
	limit      *uint64    `param:"limit"`
}

func (c *RestClient) NewGetMarginTradesRequest() *GetMarginTradesRequest {
	return &GetMarginTradesRequest{client: c}
}
