package binanceapi

import "github.com/c9s/requestgen"

// https://developers.binance.com/docs/derivatives/usds-margined-futures/account/rest-api/Toggle-BNB-Burn-On-Futures-Trade
//
//go:generate requestgen -method POST -url "/fapi/v1/feeBurn" -type FuturesToggleBnbBurnRequest -responseType FuturesToggleBnbBurnResponse
type FuturesToggleBnbBurnRequest struct {
	client requestgen.AuthenticatedAPIClient

	FeeBurn string `param:"feeBurn"` // "true" or "false"
}

type FuturesToggleBnbBurnResponse struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
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
