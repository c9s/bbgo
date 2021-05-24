package okexapi

import (
	"context"
	"net/url"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type PublicDataService struct {
	client *RestClient
}

func (s *PublicDataService) NewGetInstrumentsRequest() *GetInstrumentsRequest {
	return &GetInstrumentsRequest{
		client: s.client,
	}
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
