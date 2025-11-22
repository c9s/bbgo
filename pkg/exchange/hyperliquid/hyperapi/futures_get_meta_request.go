package hyperapi

import (
	"github.com/c9s/requestgen"
)

type FuturesGetMetaResponse struct {
	Universe []PerpMeta `json:"universe"` // universe
}

type PerpMeta struct {
	Name         string `json:"name"`
	SzDecimals   int    `json:"szDecimals"`
	MaxLeverage  int    `json:"maxLeverage"`
	OnlyIsolated bool   `json:"onlyIsolated"`
	IsDelisted   bool   `json:"isDelisted"`
}

//go:generate requestgen -method POST -url "/info" -type FuturesGetMetaRequest -responseType FuturesGetMetaResponse
type FuturesGetMetaRequest struct {
	client requestgen.APIClient

	metaType ReqTypeInfo `param:"type" default:"meta" validValues:"meta"`
}

func (c *Client) NewFuturesGetMetaRequest() *FuturesGetMetaRequest {
	return &FuturesGetMetaRequest{
		client:   c,
		metaType: ReqMeta,
	}
}
