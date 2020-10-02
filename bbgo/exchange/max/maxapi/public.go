package max

import (
	"net/url"
	"time"

	"github.com/valyala/fastjson"
)

type PublicService struct {
	client *RestClient
}

type Market struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	BaseUnit           string `json:"base_unit"`
	BaseUnitPrecision  int    `json:"base_unit_precision"`
	QuoteUnit          string `json:"quote_unit"`
	QuoteUnitPrecision int    `json:"quote_unit_precision"`
}

type Ticker struct {
	Time time.Time

	At          int64  `json:"at"`
	Buy         string `json:"buy"`
	Sell        string `json:"sell"`
	Open        string `json:"open"`
	High        string `json:"high"`
	Low         string `json:"low"`
	Last        string `json:"last"`
	Volume      string `json:"vol"`
	VolumeInBTC string `json:"vol_in_btc"`
}

func (s *PublicService) Timestamp() (serverTimestamp int64, err error) {
	// sync timestamp with server
	req, err := s.client.newRequest("GET", "v2/timestamp", nil, nil)
	if err != nil {
		return 0, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return 0, err
	}

	err = response.DecodeJSON(&serverTimestamp)
	if err != nil {
		return 0, err
	}

	return serverTimestamp, nil
}

func (s *PublicService) Markets() ([]Market, error) {
	req, err := s.client.newRequest("GET", "v2/markets", url.Values{}, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var m []Market
	if err := response.DecodeJSON(&m); err != nil {
		return nil, err
	}

	return m, nil
}

func (s *PublicService) Tickers() (map[string]Ticker, error) {
	var endPoint = "v2/tickers"
	req, err := s.client.newRequest("GET", endPoint, url.Values{}, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	v, err := fastjson.ParseBytes(response.Body)
	if err != nil {
		return nil, err
	}

	o, err := v.Object()
	if err != nil {
		return nil, err
	}

	var tickers = make(map[string]Ticker)
	o.Visit(func(key []byte, v *fastjson.Value) {
		var ticker = mustParseTicker(v)
		tickers[string(key)] = ticker
	})

	return tickers, nil
}

func (s *PublicService) Ticker(market string) (*Ticker, error) {
	var endPoint = "v2/tickers/" + market
	req, err := s.client.newRequest("GET", endPoint, url.Values{}, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	v, err := fastjson.ParseBytes(response.Body)
	if err != nil {
		return nil, err
	}

	var ticker = mustParseTicker(v)
	return &ticker, nil
}

func mustParseTicker(v *fastjson.Value) Ticker {
	var at = v.GetInt64("at")
	return Ticker{
		Time:        time.Unix(at, 0),
		At:          at,
		Buy:         string(v.GetStringBytes("buy")),
		Sell:        string(v.GetStringBytes("sell")),
		Volume:      string(v.GetStringBytes("vol")),
		VolumeInBTC: string(v.GetStringBytes("vol_in_btc")),
		Last:        string(v.GetStringBytes("last")),
		Open:        string(v.GetStringBytes("open")),
		High:        string(v.GetStringBytes("high")),
		Low:         string(v.GetStringBytes("low")),
	}
}
