package okexapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type MarketTickersRequest struct {
	client *RestClient

	instType string
}

func (r *MarketTickersRequest) InstrumentType(instType string) *MarketTickersRequest {
	r.instType = instType
	return r
}

func (r *MarketTickersRequest) Do(ctx context.Context) ([]MarketTicker, error) {
	// SPOT, SWAP, FUTURES, OPTION
	var params = url.Values{}
	params.Add("instType", string(r.instType))

	req, err := r.client.NewRequest(ctx, "GET", "/api/v5/market/tickers", params, nil)
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
	var data []MarketTicker
	if err := json.Unmarshal(apiResponse.Data, &data); err != nil {
		return nil, err
	}

	return data, nil
}

type MarketTickerRequest struct {
	client *RestClient

	instId string
}

func (r *MarketTickerRequest) InstrumentID(instId string) *MarketTickerRequest {
	r.instId = instId
	return r
}

func (r *MarketTickerRequest) Do(ctx context.Context) (*MarketTicker, error) {
	// SPOT, SWAP, FUTURES, OPTION
	var params = url.Values{}
	params.Add("instId", r.instId)

	req, err := r.client.NewRequest(ctx, "GET", "/api/v5/market/ticker", params, nil)
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
	var data []MarketTicker
	if err := json.Unmarshal(apiResponse.Data, &data); err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("ticker of %s not found", r.instId)
	}

	return &data[0], nil
}

func (c *RestClient) NewMarketTickerRequest(instId string) *MarketTickerRequest {
	return &MarketTickerRequest{
		client: c,
		instId: instId,
	}
}

func (c *RestClient) NewMarketTickersRequest(instType string) *MarketTickersRequest {
	return &MarketTickersRequest{
		client:   c,
		instType: instType,
	}
}
