package binanceapi

import "github.com/c9s/requestgen"

// https://developers.binance.com/docs/derivatives/usds-margined-futures/account/rest-api/Toggle-BNB-Burn-On-Futures-Trade
//
//go:generate requestgen -method POST -url "/fapi/v1/feeBurn" -type FuturesToggleBnbBurnRequest -responseType FuturesToggleBnbBurnResponse
type FuturesToggleBnbBurnRequest struct {
	client requestgen.AuthenticatedAPIClient

	FeeBurn string `param:"feeBurn"` // "true" or "false"
}

//go:generate requestgen -method GET -url "/fapi/v1/feeBurn" -type FuturesBnbBurnStatusRequest -responseType FuturesBnbBurnStatusResponse
type FuturesBnbBurnStatusRequest struct {
	client requestgen.AuthenticatedAPIClient
}

type FuturesToggleBnbBurnResponse struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
}

type FuturesBnbBurnStatusResponse struct {
	FeeBurn bool `json:"feeBurn"`
}

func (c *FuturesRestClient) NewFuturesToggleBnbBurnRequest(feeBurn bool) *FuturesToggleBnbBurnRequest {
	req := &FuturesToggleBnbBurnRequest{
		client: c,
	}
	if feeBurn {
		req.SetFeeBurn("true")
	} else {
		req.SetFeeBurn("false")
	}
	return req
}

func (c *FuturesRestClient) NewFuturesBnbBurnStatusRequest() *FuturesBnbBurnStatusRequest {
	req := &FuturesBnbBurnStatusRequest{
		client: c,
	}
	return req
}
