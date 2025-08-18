package okexapi

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
)

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
