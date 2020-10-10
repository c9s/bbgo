package max

import (
	"context"
	"net/url"
	"strconv"
)

type MarkerInfo struct {
	Fee         string `json:"fee"`
	FeeCurrency string `json:"fee_currency"`
	OrderID     int    `json:"order_id"`
}

type TradeInfo struct {
	// Maker tells you the maker trade side
	Maker string      `json:"maker,omitempty"`
	Bid   *MarkerInfo `json:"bid,omitempty"`
	Ask   *MarkerInfo `json:"ask,omitempty"`
}

// Trade represents one returned trade on the max platform.
type Trade struct {
	ID                    uint64    `json:"id" db:"exchange_id"`
	Price                 string    `json:"price" db:"price"`
	Volume                string    `json:"volume" db:"volume"`
	Funds                 string    `json:"funds"`
	Market                string    `json:"market" db:"market"`
	MarketName            string    `json:"market_name"`
	CreatedAt             int64     `json:"created_at"`
	CreatedAtMilliSeconds int64     `json:"created_at_in_ms"`
	Side                  string    `json:"side" db:"side"`
	OrderID               uint64    `json:"order_id"`
	Fee                   string    `json:"fee" db:"fee"` // float number as string
	FeeCurrency           string    `json:"fee_currency" db:"fee_currency"`
	Info                  TradeInfo `json:"info,omitempty"`
}

func (t Trade) IsBuyer() bool {
	return t.Side == "bid"
}

func (t Trade) IsMaker() bool {
	return t.Info.Maker == t.Side
}

type QueryTradeOptions struct {
	Market    string `json:"market"`
	Timestamp int64  `json:"timestamp,omitempty"`
	From      int64  `json:"from,omitempty"`
	To        int64  `json:"to,omitempty"`
	OrderBy   string `json:"order_by,omitempty"`
	Page      int    `json:"page,omitempty"`
	Offset    int    `json:"offset,omitempty"`
	Limit     int64  `json:"limit,omitempty"`
}

type TradeService struct {
	client *RestClient
}

func (options *QueryTradeOptions) Map() map[string]interface{} {
	var data = map[string]interface{}{}
	data["market"] = options.Market

	if options.Limit > 0 {
		data["limit"] = options.Limit
	}

	if options.Timestamp > 0 {
		data["timestamp"] = options.Timestamp
	}

	if options.From >= 0 {
		data["from"] = options.From
	}

	if options.To > options.From {
		data["to"] = options.To
	}
	if len(options.OrderBy) > 0 {
		// could be "asc" or "desc"
		data["order_by"] = options.OrderBy
	}

	return data
}

func (options *QueryTradeOptions) Params() url.Values {
	var params = url.Values{}
	params.Add("market", options.Market)

	if options.Limit > 0 {
		params.Add("limit", strconv.FormatInt(options.Limit, 10))
	}
	if options.Timestamp > 0 {
		params.Add("timestamp", strconv.FormatInt(options.Timestamp, 10))
	}
	if options.From >= 0 {
		params.Add("from", strconv.FormatInt(options.From, 10))
	}
	if options.To > options.From {
		params.Add("to", strconv.FormatInt(options.To, 10))
	}
	if len(options.OrderBy) > 0 {
		// could be "asc" or "desc"
		params.Add("order_by", options.OrderBy)
	}
	return params
}

func (s *TradeService) MyTrades(options QueryTradeOptions) ([]Trade, error) {
	req, err := s.client.newAuthenticatedRequest("GET", "v2/trades/my", options.Map())
	if err != nil {
		return nil, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var v []Trade
	if err := response.DecodeJSON(&v); err != nil {
		return nil, err
	}

	return v, nil
}

func (s *TradeService) NewPrivateTradeRequest() *PrivateTradeRequest {
	return &PrivateTradeRequest{client: s.client}
}

type PrivateRequestParams struct {
	Nonce int64  `json:"nonce"`
	Path  string `json:"path"`
}

type PrivateTradeRequestParams struct {
	*PrivateRequestParams

	Market string `json:"market"`

	// Timestamp is the seconds elapsed since Unix epoch, set to return trades executed before the time only
	Timestamp int `json:"timestamp,omitempty"`

	// From field is a trade id, set ot return trades created after the trade
	From int64 `json:"from,omitempty"`

	// To field trade id, set to return trades created before the trade
	To int64 `json:"to,omitempty"`

	OrderBy string `json:"order_by,omitempty"`

	// default to false
	Pagination bool `json:"pagination"`

	Limit int64 `json:"limit,omitempty"`

	Offset int64 `json:"offset,omitempty"`
}

type PrivateTradeRequest struct {
	client *RestClient
	params PrivateTradeRequestParams
}

func (r *PrivateTradeRequest) Market(market string) *PrivateTradeRequest {
	r.params.Market = market
	return r
}

func (r *PrivateTradeRequest) From(from int64) *PrivateTradeRequest {
	r.params.From = from
	return r
}

func (r *PrivateTradeRequest) To(to int64) *PrivateTradeRequest {
	r.params.To = to
	return r
}

func (r *PrivateTradeRequest) Limit(limit int64) *PrivateTradeRequest {
	r.params.Limit = limit
	return r
}

func (r *PrivateTradeRequest) Offset(offset int64) *PrivateTradeRequest {
	r.params.Offset = offset
	return r
}

func (r *PrivateTradeRequest) Pagination(p bool) *PrivateTradeRequest {
	r.params.Pagination = p
	return r
}

func (r *PrivateTradeRequest) OrderBy(orderBy string) *PrivateTradeRequest {
	r.params.OrderBy = orderBy
	return r
}

func (r *PrivateTradeRequest) Do(ctx context.Context) (trades []Trade, err error) {
	req, err := r.client.newAuthenticatedRequest("GET", "v2/trades/my", &r.params)
	if err != nil {
		return trades, err
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return trades, err
	}

	if err := response.DecodeJSON(&trades); err != nil {
		return trades, err
	}

	return trades, err
}

func (s *TradeService) Trades(options QueryTradeOptions) ([]Trade, error) {
	var params = options.Params()

	req, err := s.client.newRequest("GET", "v2/trades", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var v []Trade
	if err := response.DecodeJSON(&v); err != nil {
		return nil, err
	}

	return v, nil
}
