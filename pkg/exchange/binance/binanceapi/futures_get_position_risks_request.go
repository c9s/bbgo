package binanceapi

import "github.com/c9s/requestgen"

type FuturesPositionRisksResponse struct {
}

//go:generate requestgen -method GET -url "/fapi/v2/positionRisk" -type FuturesGetPositionRisksRequest -responseType .FuturesPositionRisksResponse
type FuturesGetPositionRisksRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol"`
}

func (c *FuturesRestClient) NewGetPositionRisksRequest() *FuturesGetPositionRisksRequest {
	return &FuturesGetPositionRisksRequest{client: c}
}
