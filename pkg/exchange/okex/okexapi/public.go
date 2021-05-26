package okexapi

import (
	"context"
	"net/url"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
)

type PublicDataService struct {
	client *RestClient
}

func (s *PublicDataService) NewGetInstrumentsRequest() *GetInstrumentsRequest {
	return &GetInstrumentsRequest{
		client: s.client,
	}
}

func (s *PublicDataService) NewGetFundingRate() *GetFundingRateRequest {
	return &GetFundingRateRequest{
		client: s.client,
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

	req, err := r.client.newRequest("GET", "/api/v5/public/funding-rate", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string        `json:"code"`
		Message string        `json:"msg"`
		Data    []FundingRate `json:"data"`
	}
	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	if len(apiResponse.Data) == 0 {
		return nil, errors.New("empty funding rate data")
	}

	return &apiResponse.Data[0], nil
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

	req, err := r.client.newRequest("GET", "/api/v5/public/instruments", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string       `json:"code"`
		Message string       `json:"msg"`
		Data    []Instrument `json:"data"`
	}
	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	return apiResponse.Data, nil
}
