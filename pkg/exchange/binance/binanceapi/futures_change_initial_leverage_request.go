package binanceapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type FuturesChangeInitialLeverageResponse struct {
	Leverage         int              `json:"leverage"`
	MaxNotionalValue fixedpoint.Value `json:"maxNotionalValue"`
	Symbol           string           `json:"symbol"`
}

//go:generate requestgen -method POST -url "/fapi/v1/leverage" -type FuturesChangeInitialLeverageRequest -responseType FuturesChangeInitialLeverageResponse
type FuturesChangeInitialLeverageRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol   string `param:"symbol"`
	leverage int    `param:"leverage"`
}

func (c *FuturesRestClient) NewFuturesChangeInitialLeverageRequest() *FuturesChangeInitialLeverageRequest {
	return &FuturesChangeInitialLeverageRequest{client: c}
}
