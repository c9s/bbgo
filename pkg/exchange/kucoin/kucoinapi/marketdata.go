package kucoinapi

import (
	"net/url"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
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

	req, err := s.client.newRequest("GET", "/api/v1/symbols", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.sendRequest(req)
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

	req, err := s.client.newRequest("GET", "/api/v1/market/orderbook/level1", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string   `json:"code"`
		Message string   `json:"msg"`
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
            "symbolName":"BTC-USDT", // Name of trading pairs, it would change after renaming
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
	Symbol string `json:"symbol"`
	Name string `json:"symbolName"`
	Buy fixedpoint.Value `json:"buy"`
	Sell fixedpoint.Value `json:"sell"`
	ChangeRate fixedpoint.Value `json:"changeRate"`
	ChangePrice fixedpoint.Value `json:"changePrice"`
	High fixedpoint.Value `json:"high"`
	Low fixedpoint.Value `json:"low"`
	Last fixedpoint.Value `json:"last"`
	AveragePrice fixedpoint.Value `json:"averagePrice"`
	Volume fixedpoint.Value `json:"vol"` // base volume
	VolumeValue fixedpoint.Value `json:"volValue"` // quote volume

	TakerFeeRate fixedpoint.Value `json:"takerFeeRate"`
	MakerFeeRate fixedpoint.Value `json:"makerFeeRate"`

	TakerCoefficient fixedpoint.Value `json:"takerCoefficient"`
	MakerCoefficient fixedpoint.Value `json:"makerCoefficient"`
}

type AllTickers struct {
	Time types.MillisecondTimestamp `json:"time"`
	Ticker []Ticker24H `json:"ticker"`
}


func (s *MarketDataService) ListTickers() (*AllTickers, error) {
	req, err := s.client.newRequest("GET", "/api/v1/market/allTickers", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string   `json:"code"`
		Message string   `json:"msg"`
		Data    *AllTickers `json:"data"`
	}

	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	return apiResponse.Data, nil
}

