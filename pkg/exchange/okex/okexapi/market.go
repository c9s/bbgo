package okexapi

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Candle struct {
	InstrumentID     string
	Interval         string
	Time             time.Time
	Open             fixedpoint.Value
	High             fixedpoint.Value
	Low              fixedpoint.Value
	Close            fixedpoint.Value
	Volume           fixedpoint.Value
	VolumeInCurrency fixedpoint.Value
}

type CandlesticksRequest struct {
	client *RestClient

	instId string `param:"instId"`

	limit *int `param:"limit"`

	bar *string `param:"bar"`

	after *int64 `param:"after,seconds"`

	before *int64 `param:"before,seconds"`
}

func (r *CandlesticksRequest) After(after int64) *CandlesticksRequest {
	r.after = &after
	return r
}

func (r *CandlesticksRequest) Before(before int64) *CandlesticksRequest {
	r.before = &before
	return r
}

func (r *CandlesticksRequest) Bar(bar string) *CandlesticksRequest {
	r.bar = &bar
	return r
}

func (r *CandlesticksRequest) Limit(limit int) *CandlesticksRequest {
	r.limit = &limit
	return r
}

func (r *CandlesticksRequest) InstrumentID(instId string) *CandlesticksRequest {
	r.instId = instId
	return r
}

func (r *CandlesticksRequest) Do(ctx context.Context) ([]Candle, error) {
	// SPOT, SWAP, FUTURES, OPTION
	var params = url.Values{}
	params.Add("instId", r.instId)

	if r.bar != nil {
		params.Add("bar", *r.bar)
	}

	if r.before != nil {
		params.Add("before", strconv.FormatInt(*r.before, 10))
	}

	if r.after != nil {
		params.Add("after", strconv.FormatInt(*r.after, 10))
	}

	if r.limit != nil {
		params.Add("limit", strconv.Itoa(*r.limit))
	}

	req, err := r.client.newRequest("GET", "/api/v5/market/candles", params, nil)
	if err != nil {
		return nil, err
	}

	resp, err := r.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	type candleEntry [7]string
	var candlesResponse struct {
		Code    string        `json:"code"`
		Message string        `json:"msg"`
		Data    []candleEntry `json:"data"`
	}

	if err := resp.DecodeJSON(&candlesResponse); err != nil {
		return nil, err
	}

	var candles []Candle
	for _, entry := range candlesResponse.Data {
		timestamp, err := strconv.ParseInt(entry[0], 10, 64)
		if err != nil {
			return candles, err
		}

		open, err := fixedpoint.NewFromString(entry[1])
		if err != nil {
			return candles, err
		}

		high, err := fixedpoint.NewFromString(entry[2])
		if err != nil {
			return candles, err
		}

		low, err := fixedpoint.NewFromString(entry[3])
		if err != nil {
			return candles, err
		}

		cls, err := fixedpoint.NewFromString(entry[4])
		if err != nil {
			return candles, err
		}

		vol, err := fixedpoint.NewFromString(entry[5])
		if err != nil {
			return candles, err
		}

		volCcy, err := fixedpoint.NewFromString(entry[6])
		if err != nil {
			return candles, err
		}

		var interval = "1m"
		if r.bar != nil {
			interval = *r.bar
		}

		candles = append(candles, Candle{
			InstrumentID:     r.instId,
			Interval:         interval,
			Time:             time.Unix(0, timestamp*int64(time.Millisecond)),
			Open:             open,
			High:             high,
			Low:              low,
			Close:            cls,
			Volume:           vol,
			VolumeInCurrency: volCcy,
		})
	}

	return candles, nil
}

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

	req, err := r.client.newRequest("GET", "/api/v5/market/tickers", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var tickerResponse struct {
		Code    string         `json:"code"`
		Message string         `json:"msg"`
		Data    []MarketTicker `json:"data"`
	}

	if err := response.DecodeJSON(&tickerResponse); err != nil {
		return nil, err
	}

	return tickerResponse.Data, nil
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

	req, err := r.client.newRequest("GET", "/api/v5/market/ticker", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var tickerResponse struct {
		Code    string         `json:"code"`
		Message string         `json:"msg"`
		Data    []MarketTicker `json:"data"`
	}
	if err := response.DecodeJSON(&tickerResponse); err != nil {
		return nil, err
	}

	if len(tickerResponse.Data) == 0 {
		return nil, fmt.Errorf("ticker of %s not found", r.instId)
	}

	return &tickerResponse.Data[0], nil
}

type MarketDataService struct {
	client *RestClient
}

func (c *MarketDataService) NewMarketTickerRequest(instId string) *MarketTickerRequest {
	return &MarketTickerRequest{
		client: c.client,
		instId: instId,
	}
}

func (c *MarketDataService) NewMarketTickersRequest(instType string) *MarketTickersRequest {
	return &MarketTickersRequest{
		client:   c.client,
		instType: instType,
	}
}

func (c *MarketDataService) NewCandlesticksRequest(instId string) *CandlesticksRequest {
	return &CandlesticksRequest{
		client: c.client,
		instId: instId,
	}
}
