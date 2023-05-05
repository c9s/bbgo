package binanceapi

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

type FuturesOpenInterest struct {
	OpenInterest fixedpoint.Value           `json:"openInterest"`
	Symbol       string                     `json:"symbol"`
	Time         types.MillisecondTimestamp `json:"time"`
}

//go:generate requestgen -method GET -url "/fapi/v1/openInterest" -type FuturesGetOpenInterestRequest -responseType FuturesOpenInterest
type FuturesGetOpenInterestRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol"`
}

func (c *FuturesRestClient) NewFuturesGetOpenInterestRequest() *FuturesGetOpenInterestRequest {
	return &FuturesGetOpenInterestRequest{client: c}
}
