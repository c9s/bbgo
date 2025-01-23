package okexapi

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type PositionRiskResponse struct {
	AdjEq   string `json:"adjEq"`
	BalData []struct {
		Ccy   string `json:"ccy"`
		DisEq string `json:"disEq"`
		Eq    string `json:"eq"`
	} `json:"balData"`
	PosData []struct {
		BaseBal     string `json:"baseBal"`
		Ccy         string `json:"ccy"`
		InstId      string `json:"instId"`
		InstType    string `json:"instType"`
		MgnMode     string `json:"mgnMode"`
		NotionalCcy string `json:"notionalCcy"`
		NotionalUsd string `json:"notionalUsd"`
		Pos         string `json:"pos"`
		PosCcy      string `json:"posCcy"`
		PosId       string `json:"posId"`
		PosSide     string `json:"posSide"`
		QuoteBal    string `json:"quoteBal"`
	} `json:"posData"`
	Ts string `json:"ts"`
}

//go:generate GetRequest -url "/api/v5/account/account-position-risk" -type GetAccountPositionRiskRequest -responseDataType []PositionRiskResponse
type GetAccountPositionRiskRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetAccountPositionRiskRequest() *GetAccountPositionRiskRequest {
	return &GetAccountPositionRiskRequest{
		client: c,
	}
}
