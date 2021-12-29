package kucoinapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/valyala/fastjson"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type MarketDataService struct {
	client *RestClient
}

func (s *MarketDataService) NewGetKLinesRequest() *GetKLinesRequest {
	return &GetKLinesRequest{client: s.client}
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

	req, err := s.client.NewRequest(context.Background(), "GET", "/api/v1/symbols", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string   `json:"code"`
		Message string   `json:"msg"`
		Data    []Symbol `json:"data"`
	}

	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	return apiResponse.Data, nil
}

/*
//Get Ticker
{
    "sequence": "1550467636704",
    "bestAsk": "0.03715004",
    "size": "0.17",
    "price": "0.03715005",
    "bestBidSize": "3.803",
    "bestBid": "0.03710768",
    "bestAskSize": "1.788",
    "time": 1550653727731
}
*/
type Ticker struct {
	Sequence    string                     `json:"sequence"`
	Size        fixedpoint.Value           `json:"size"`
	Price       fixedpoint.Value           `json:"price"`
	BestAsk     fixedpoint.Value           `json:"bestAsk"`
	BestBid     fixedpoint.Value           `json:"bestBid"`
	BestBidSize fixedpoint.Value           `json:"bestBidSize"`
	Time        types.MillisecondTimestamp `json:"time"`
}

func (s *MarketDataService) GetTicker(symbol string) (*Ticker, error) {
	var params = url.Values{}
	params["symbol"] = []string{symbol}

	req, err := s.client.NewRequest(context.Background(), "GET", "/api/v1/market/orderbook/level1", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string  `json:"code"`
		Message string  `json:"msg"`
		Data    *Ticker `json:"data"`
	}

	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	return apiResponse.Data, nil
}

/*
{
    "time":1602832092060,
    "ticker":[
        {
            "symbol": "BTC-USDT",   // symbol
            "symbolName":"BTC-USDT", // SymbolName of trading pairs, it would change after renaming
            "buy": "11328.9",   // bestAsk
            "sell": "11329",    // bestBid
            "changeRate": "-0.0055",    // 24h change rate
            "changePrice": "-63.6", // 24h change price
            "high": "11610",    // 24h highest price
            "low": "11200", // 24h lowest price
            "vol": "2282.70993217", // 24h volume，the aggregated trading volume in BTC
            "volValue": "25984946.157790431",   // 24h total, the trading volume in quote currency of last 24 hours
            "last": "11328.9",  // last price
            "averagePrice": "11360.66065903",   // 24h average transaction price yesterday
            "takerFeeRate": "0.001",    // Basic Taker Fee
            "makerFeeRate": "0.001",    // Basic Maker Fee
            "takerCoefficient": "1",    // Taker Fee Coefficient
            "makerCoefficient": "1" // Maker Fee Coefficient
        }
    ]
}
*/

type Ticker24H struct {
	Symbol       string           `json:"symbol"`
	SymbolName   string           `json:"symbolName"`
	Buy          fixedpoint.Value `json:"buy"`
	Sell         fixedpoint.Value `json:"sell"`
	ChangeRate   fixedpoint.Value `json:"changeRate"`
	ChangePrice  fixedpoint.Value `json:"changePrice"`
	High         fixedpoint.Value `json:"high"`
	Low          fixedpoint.Value `json:"low"`
	Last         fixedpoint.Value `json:"last"`
	AveragePrice fixedpoint.Value `json:"averagePrice"`
	Volume       fixedpoint.Value `json:"vol"`      // base volume
	VolumeValue  fixedpoint.Value `json:"volValue"` // quote volume

	TakerFeeRate fixedpoint.Value `json:"takerFeeRate"`
	MakerFeeRate fixedpoint.Value `json:"makerFeeRate"`

	TakerCoefficient fixedpoint.Value `json:"takerCoefficient"`
	MakerCoefficient fixedpoint.Value `json:"makerCoefficient"`

	Time types.MillisecondTimestamp `json:"time"`
}

type AllTickers struct {
	Time   types.MillisecondTimestamp `json:"time"`
	Ticker []Ticker24H                `json:"ticker"`
}

func (s *MarketDataService) ListTickers() (*AllTickers, error) {
	req, err := s.client.NewRequest(context.Background(), "GET", "/api/v1/market/allTickers", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string      `json:"code"`
		Message string      `json:"msg"`
		Data    *AllTickers `json:"data"`
	}

	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	return apiResponse.Data, nil
}

func (s *MarketDataService) GetTicker24HStat(symbol string) (*Ticker24H, error) {
	var params = url.Values{}
	params.Add("symbol", symbol)

	req, err := s.client.NewRequest(context.Background(), "GET", "/api/v1/market/stats", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string     `json:"code"`
		Message string     `json:"msg"`
		Data    *Ticker24H `json:"data"`
	}

	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	return apiResponse.Data, nil
}

/*
{
    "sequence": "3262786978",
    "time": 1550653727731,
    "bids": [["6500.12", "0.45054140"],
             ["6500.11", "0.45054140"]],  //[price，size]
    "asks": [["6500.16", "0.57753524"],
             ["6500.15", "0.57753524"]]
}
*/
type OrderBook struct {
	Sequence string                     `json:"sequence,omitempty"`
	Time     types.MillisecondTimestamp `json:"time"`
	Bids     types.PriceVolumeSlice     `json:"bids,omitempty"`
	Asks     types.PriceVolumeSlice     `json:"asks,omitempty"`
}

func (s *MarketDataService) GetOrderBook(symbol string, depth int) (*OrderBook, error) {
	params := url.Values{}
	params["symbol"] = []string{symbol}

	var req *http.Request
	var err error

	switch depth {
	case 20, 100:
		refURL := "/api/v1/market/orderbook/level2_" + strconv.Itoa(depth)
		req, err = s.client.NewRequest(context.Background(), "GET", refURL, params, nil)
		if err != nil {
			return nil, err
		}

	case 0:
		refURL := "/api/v3/market/orderbook/level2"
		req, err = s.client.NewAuthenticatedRequest(context.Background(), "GET", refURL, params, nil)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("depth %d is not supported, use 20, 100 or 0", depth)

	}

	response, err := s.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string     `json:"code"`
		Message string     `json:"msg"`
		Data    *OrderBook `json:"data"`
	}

	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	return apiResponse.Data, nil
}

//go:generate requestgen -type GetKLinesRequest
type GetKLinesRequest struct {
	client *RestClient

	symbol string `param:"symbol"`

	interval string `param:"type" validValues:"1min,3min,5min,15min,30min,1hour,2hour,4hour,6hour,8hour,12hour,1day,1week"`

	startAt *time.Time `param:"startAt,seconds"`

	endAt *time.Time `param:"endAt,seconds"`
}

type KLine struct {
	Symbol              string
	Interval            string
	StartTime           time.Time
	Open                fixedpoint.Value
	High                fixedpoint.Value
	Low                 fixedpoint.Value
	Close               fixedpoint.Value
	Volume, QuoteVolume fixedpoint.Value
}

func (r *GetKLinesRequest) Do(ctx context.Context) ([]KLine, error) {
	params, err := r.GetParametersQuery()
	if err != nil {
		return nil, err
	}

	req, err := r.client.NewRequest(ctx, "GET", "/api/v1/market/candles", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := r.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string          `json:"code"`
		Message string          `json:"msg"`
		Data    json.RawMessage `json:"data"`
	}

	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	if apiResponse.Data == nil {
		return nil, errors.New("api error: [" + apiResponse.Code + "] " + apiResponse.Message)
	}

	return parseKLines(apiResponse.Data, r.symbol, r.interval)
}

func parseKLines(b []byte, symbol, interval string) (klines []KLine, err error) {
	s, err := fastjson.ParseBytes(b)
	if err != nil {
		return klines, err
	}

	for _, v := range s.GetArray() {
		arr := v.GetArray()
		ts, err := strconv.ParseInt(string(arr[0].GetStringBytes()), 10, 64)
		if err != nil {
			return klines, err
		}

		o, err := fixedpoint.NewFromString(string(arr[1].GetStringBytes()))
		if err != nil {
			return klines, err
		}

		c, err := fixedpoint.NewFromString(string(arr[2].GetStringBytes()))
		if err != nil {
			return klines, err
		}

		h, err := fixedpoint.NewFromString(string(arr[3].GetStringBytes()))
		if err != nil {
			return klines, err
		}

		l, err := fixedpoint.NewFromString(string(arr[4].GetStringBytes()))
		if err != nil {
			return klines, err
		}

		vv, err := fixedpoint.NewFromString(string(arr[5].GetStringBytes()))
		if err != nil {
			return klines, err
		}

		qv, err := fixedpoint.NewFromString(string(arr[6].GetStringBytes()))
		if err != nil {
			return klines, err
		}

		klines = append(klines, KLine{
			Symbol:      symbol,
			Interval:    interval,
			StartTime:   time.Unix(ts, 0),
			Open:        o,
			High:        h,
			Low:         l,
			Close:       c,
			Volume:      vv,
			QuoteVolume: qv,
		})
	}

	return klines, err
}
