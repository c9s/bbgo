package okexapi

import (
	"time"

	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

//go:generate GetRequest -url "/api/v5/trade/fills" -type GetThreeDaysTransactionHistoryRequest -responseDataType []Trade -rateLimiter 1+60/2s
type GetThreeDaysTransactionHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	instrumentType InstrumentType `param:"instType,query"`
	instrumentID   *string        `param:"instId,query"`
	orderID        *string        `param:"ordId,query"`

	underlying       *string `param:"uly,query"`
	instrumentFamily *string `param:"instFamily,query"`

	after     *string    `param:"after,query"`
	before    *string    `param:"before,query"`
	startTime *time.Time `param:"begin,query,milliseconds"`

	// endTime for each request, startTime and endTime can be any interval, but should be in last 3 months
	endTime *time.Time `param:"end,query,milliseconds"`

	// limit for data size per page. Default: 100
	limit *uint64 `param:"limit,query"`
}

func (c *RestClient) NewGetThreeDaysTransactionHistoryRequest() *GetThreeDaysTransactionHistoryRequest {
	return &GetThreeDaysTransactionHistoryRequest{
		client:         c,
		instrumentType: InstrumentTypeSpot,
	}
}
