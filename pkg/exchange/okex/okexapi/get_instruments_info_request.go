package okexapi

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type InstrumentInfo struct {
	InstrumentType        string                     `json:"instType"`
	InstrumentID          string                     `json:"instId"`
	BaseCurrency          string                     `json:"baseCcy"`
	QuoteCurrency         string                     `json:"quoteCcy"`
	SettleCurrency        string                     `json:"settleCcy"`
	ContractValue         string                     `json:"ctVal"`
	ContractMultiplier    string                     `json:"ctMult"`
	ContractValueCurrency string                     `json:"ctValCcy"`
	ListTime              types.MillisecondTimestamp `json:"listTime"`
	ExpiryTime            types.MillisecondTimestamp `json:"expTime"`
	TickSize              fixedpoint.Value           `json:"tickSz"`
	LotSize               fixedpoint.Value           `json:"lotSz"`

	// MinSize = min order size
	MinSize fixedpoint.Value `json:"minSz"`

	// instrument status
	State string `json:"state"`
}

//go:generate GetRequest -url "/api/v5/public/instruments" -type GetInstrumentsInfoRequest -responseDataType []InstrumentInfo
type GetInstrumentsInfoRequest struct {
	client requestgen.APIClient

	instType InstrumentType `param:"instType,query" validValues:"SPOT"`

	instId *string `param:"instId,query"`
}

func (c *RestClient) NewGetInstrumentsInfoRequest() *GetInstrumentsInfoRequest {
	return &GetInstrumentsInfoRequest{
		client:   c,
		instType: InstrumentTypeSpot,
	}
}
