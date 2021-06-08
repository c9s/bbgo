package max

import (
	"context"
	"net/url"
	"strconv"

	"github.com/pkg/errors"
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
	return t.Side == "bid" || t.Side == "buy"
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

func (s *TradeService) NewPrivateTradeRequest() *PrivateTradeRequest {
	return &PrivateTradeRequest{client: s.client}
}

type PrivateRequestParams struct {
	Nonce int64  `json:"nonce"`
	Path  string `json:"path"`
}

type PrivateTradeRequest struct {
	client *RestClient

	market *string

	// Timestamp is the seconds elapsed since Unix epoch, set to return trades executed before the time only
	timestamp *int64

	// From field is a trade id, set ot return trades created after the trade
	from *int64

	// To field trade id, set to return trades created before the trade
	to *int64

	orderBy *string

	pagination *bool

	limit *int64

	offset *int64
}

func (r *PrivateTradeRequest) Market(market string) *PrivateTradeRequest {
	r.market = &market
	return r
}

func (r *PrivateTradeRequest) From(from int64) *PrivateTradeRequest {
	r.from = &from
	return r
}

func (r *PrivateTradeRequest) Timestamp(t int64) *PrivateTradeRequest {
	r.timestamp = &t
	return r
}

func (r *PrivateTradeRequest) To(to int64) *PrivateTradeRequest {
	r.to = &to
	return r
}

func (r *PrivateTradeRequest) Limit(limit int64) *PrivateTradeRequest {
	r.limit = &limit
	return r
}

func (r *PrivateTradeRequest) Offset(offset int64) *PrivateTradeRequest {
	r.offset = &offset
	return r
}

func (r *PrivateTradeRequest) Pagination(p bool) *PrivateTradeRequest {
	r.pagination = &p
	return r
}

func (r *PrivateTradeRequest) OrderBy(orderBy string) *PrivateTradeRequest {
	r.orderBy = &orderBy
	return r
}

func (r *PrivateTradeRequest) Do(ctx context.Context) (trades []Trade, err error) {
	if r.market == nil {
		return nil, errors.New("parameter market is mandatory")
	}

	payload := map[string]interface{}{
		"market": r.market,
	}

	if r.timestamp != nil {
		payload["timestamp"] = r.timestamp
	}

	if r.from != nil {
		payload["from"] = r.from
	}

	if r.to != nil {
		payload["to"] = r.to
	}

	if r.orderBy != nil {
		payload["order_by"] = r.orderBy
	}

	if r.pagination != nil {
		payload["pagination"] = r.pagination
	}

	if r.limit != nil {
		payload["limit"] = r.limit
	}

	if r.offset != nil {
		payload["offset"] = r.offset
	}

	req, err := r.client.newAuthenticatedRequest("GET", "v2/trades/my", payload, nil)
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

