package kucoinapi

import (
	"net/url"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/pkg/errors"
)

type MarketDataService struct {
	client *RestClient
}

type Symbol struct {
	Symbol          string           `json:"symbol"`
	Name            string           `json:"name"`
	BaseCurrency    string           `json:"baseCurrency"`
	QuoteCurrency   string           `json:"quoteCurrency"`
	FeeCurrency     string           `json:"feeCurrency"`
	Market          string           `json:"market"`
	BaseMinSize     fixedpoint.Value `json:"baseMinSize"`
	QuoteMinSize    fixedpoint.Value `json:"quoteMinSize"`
	BaseIncrement   fixedpoint.Value `json:"baseIncrement"`
	QuoteIncrement  fixedpoint.Value `json:"quoteIncrement"`
	PriceIncrement  fixedpoint.Value `json:"priceIncrement"`
	PriceLimitRate  fixedpoint.Value `json:"priceLimitRate"`
	IsMarginEnabled bool             `json:"isMarginEnabled"`
	EnableTrading   bool             `json:"enableTrading"`
}

func (s *MarketDataService) ListSymbols(market ...string) ([]Symbol, error) {
	var params = url.Values{}

	if len(market) == 1 {
		params["market"] = []string{market[0]}
	} else if len(market) > 1 {
		return nil, errors.New("symbols api only supports one market parameter")
	}

	req, err := s.client.newRequest("GET", "/api/v1/symbols", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string       `json:"code"`
		Message string       `json:"msg"`
		Data    []Symbol `json:"data"`
	}

	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	return apiResponse.Data, nil
}
