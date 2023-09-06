package okexapi

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
)

func (s *RestClient) NewGetInstrumentsRequest() *GetInstrumentsRequest {
	return &GetInstrumentsRequest{
		client: s,
	}
}

func (s *RestClient) NewGetFundingRate() *GetFundingRateRequest {
	return &GetFundingRateRequest{
		client: s,
	}
}

type FundingRate struct {
	InstrumentType  string                     `json:"instType"`
	InstrumentID    string                     `json:"instId"`
	FundingRate     fixedpoint.Value           `json:"fundingRate"`
	NextFundingRate fixedpoint.Value           `json:"nextFundingRate"`
	FundingTime     types.MillisecondTimestamp `json:"fundingTime"`
}

type GetFundingRateRequest struct {
	client *RestClient

	instId string
}

func (r *GetFundingRateRequest) InstrumentID(instId string) *GetFundingRateRequest {
	r.instId = instId
	return r
}

func (r *GetFundingRateRequest) Do(ctx context.Context) (*FundingRate, error) {
	// SPOT, SWAP, FUTURES, OPTION
	var params = url.Values{}
	params.Add("instId", string(r.instId))

	req, err := r.client.NewRequest(ctx, "GET", "/api/v5/public/funding-rate", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := r.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse APIResponse
	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}
	var data []FundingRate
	if err := json.Unmarshal(apiResponse.Data, &data); err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, errors.New("empty funding rate data")
	}

	return &data[0], nil
}

type Instrument struct {
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

type GetInstrumentsRequest struct {
	client *RestClient

	instType InstrumentType

	instId *string
}

func (r *GetInstrumentsRequest) InstrumentType(instType InstrumentType) *GetInstrumentsRequest {
	r.instType = instType
	return r
}

func (r *GetInstrumentsRequest) InstrumentID(instId string) *GetInstrumentsRequest {
	r.instId = &instId
	return r
}

func (r *GetInstrumentsRequest) Do(ctx context.Context) ([]Instrument, error) {
	// SPOT, SWAP, FUTURES, OPTION
	var params = url.Values{}
	params.Add("instType", string(r.instType))

	if r.instId != nil {
		params.Add("instId", *r.instId)
	}

	req, err := r.client.NewRequest(ctx, "GET", "/api/v5/public/instruments", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := r.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse APIResponse
	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}
	var data []Instrument
	if err := json.Unmarshal(apiResponse.Data, &data); err != nil {
		return nil, err
	}

	return data, nil
}
