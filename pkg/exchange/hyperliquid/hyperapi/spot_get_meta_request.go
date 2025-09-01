package hyperapi

import (
	"github.com/c9s/requestgen"
)

type SpotGetMetaResponse struct {
	Tokens   []TokenMeta    `json:"tokens"`
	Universe []UniverseMeta `json:"universe"`
}

type TokenMeta struct {
	Name        string `json:"name"`
	SzDecimals  int    `json:"szDecimals"`
	WeiDecimals int    `json:"weiDecimals"`
	Index       int    `json:"index"`
	TokenId     string `json:"tokenId"`
	IsCanonical bool   `json:"isCanonical"`
	EvmContract string `json:"evmContract"`
	FullName    string `json:"fullName"`
}

type UniverseMeta struct {
	Name        string `json:"name"`
	Tokens      [2]int `json:"tokens"`
	Index       int    `json:"index"`
	IsCanonical bool   `json:"isCanonical"`
}

//go:generate requestgen -method POST -url "/info" -type SpotGetMetaRequest -responseType SpotGetMetaResponse
type SpotGetMetaRequest struct {
	client requestgen.APIClient

	metaType InfoReqType `param:"type" default:"spotMeta" validValues:"spotMeta"`
}

func (c *Client) NewGetSpotGetMetaRequest() *SpotGetMetaRequest {
	return &SpotGetMetaRequest{
		client:   c,
		metaType: SpotMeta,
	}
}
