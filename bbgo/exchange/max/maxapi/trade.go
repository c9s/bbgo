package max

import (
	"net/url"
	"strconv"
	"time"
)

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
	OrderID               uint64    `json:"order_id" db:"order_id"`
	Fee                   string    `json:"fee" db:"fee"` // float number as string
	FeeCurrency           string    `json:"fee_currency" db:"fee_currency"`
	CreatedAtInDB         time.Time `db:"created_at"`
	InsertedAt            time.Time `db:"inserted_at"`
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

