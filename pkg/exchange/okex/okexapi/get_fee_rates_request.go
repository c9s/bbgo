package okexapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type FeeRateList []FeeRate

type FeeRate struct {
	Delivery       string                     `json:"delivery"`
	Exercise       string                     `json:"exercise"`
	InstrumentType InstrumentType             `json:"instType"`
	Level          string                     `json:"level"`
	Maker          fixedpoint.Value           `json:"maker"`
	MakerU         string                     `json:"makerU"`
	MakerUSDC      string                     `json:"makerUSDC"`
	Taker          fixedpoint.Value           `json:"taker"`
	TakerU         string                     `json:"takerU"`
	TakerUSDC      string                     `json:"takerUSDC"`
	Timestamp      types.MillisecondTimestamp `json:"ts"`
}

//go:generate GetRequest -url "/api/v5/account/trade-fee" -type GetFeeRatesRequest -responseDataType .FeeRateList
type GetFeeRatesRequest struct {
	client requestgen.AuthenticatedAPIClient

	instrumentType InstrumentType `param:"instType,query"`
	instrumentID   *string        `param:"instId,query"`
	// Underlying and InstrumentFamily Applicable to FUTURES/SWAP/OPTION
	underlying       *string `param:"uly,query"`
	instrumentFamily *string `param:"instFamily,query"`
}

func (c *RestClient) NewGetFeeRatesRequest() *GetFeeRatesRequest {
	return &GetFeeRatesRequest{
		client:         c,
		instrumentType: InstrumentTypeSpot,
	}
}
