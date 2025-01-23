package okexapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type PositionData struct {
	Currency string `json:"ccy"`
	InstId   string `json:"instId"`
	InstType string `json:"instType"`

	MarginMode       MarginMode       `json:"mgnMode"`
	NotionalCurrency fixedpoint.Value `json:"notionalCcy"`
	NotionalUsd      fixedpoint.Value `json:"notionalUsd"`
	Pos              fixedpoint.Value `json:"pos"`
	PosCurrency      string           `json:"posCcy"`
	PosId            string           `json:"posId"`
	PosSide          string           `json:"posSide"`

	BaseBal  fixedpoint.Value `json:"baseBal"`
	QuoteBal fixedpoint.Value `json:"quoteBal"`
}

type PositionBalanceData struct {
	Currency  string           `json:"ccy"`
	DisEquity fixedpoint.Value `json:"disEq"`
	Equity    fixedpoint.Value `json:"eq"`
}

type PositionRiskResponse struct {
	AdjEq   fixedpoint.Value           `json:"adjEq"`
	BalData []PositionBalanceData      `json:"balData"`
	PosData []PositionData             `json:"posData"`
	Ts      types.MillisecondTimestamp `json:"ts"`
}

//go:generate GetRequest -url "/api/v5/account/account-position-risk" -type GetAccountPositionRiskRequest -responseDataType []PositionRiskResponse
type GetAccountPositionRiskRequest struct {
	client requestgen.AuthenticatedAPIClient

	instType *InstrumentType `param:"instType"`
}

func (c *RestClient) NewGetAccountPositionRiskRequest() *GetAccountPositionRiskRequest {
	return &GetAccountPositionRiskRequest{
		client: c,
	}
}
