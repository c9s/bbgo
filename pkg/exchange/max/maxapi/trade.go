package max

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST

import (
	"net/url"
	"strconv"
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
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

type Liquidity string

// Trade represents one returned trade on the max platform.
type Trade struct {
	ID          uint64                     `json:"id" db:"exchange_id"`
	WalletType  WalletType                 `json:"wallet_type,omitempty"`
	Price       fixedpoint.Value           `json:"price"`
	Volume      fixedpoint.Value           `json:"volume"`
	Funds       fixedpoint.Value           `json:"funds"`
	Market      string                     `json:"market"`
	MarketName  string                     `json:"market_name"`
	CreatedAt   types.MillisecondTimestamp `json:"created_at"`
	Side        string                     `json:"side"`
	OrderID     uint64                     `json:"order_id"`
	Fee         fixedpoint.Value           `json:"fee"` // float number as string
	FeeCurrency string                     `json:"fee_currency"`
	Liquidity   Liquidity                  `json:"liquidity"`
	Info        TradeInfo                  `json:"info,omitempty"`
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
	client requestgen.AuthenticatedAPIClient
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

func (s *TradeService) NewGetPrivateTradeRequest() *GetPrivateTradesRequest {
	return &GetPrivateTradesRequest{client: s.client}
}

type PrivateRequestParams struct {
	Nonce int64  `json:"nonce"`
	Path  string `json:"path"`
}

//go:generate GetRequest -url "v2/trades/my" -type GetPrivateTradesRequest -responseType []Trade
type GetPrivateTradesRequest struct {
	client requestgen.AuthenticatedAPIClient

	market string `param:"market"` // nolint:golint,structcheck

	// timestamp is the seconds elapsed since Unix epoch, set to return trades executed before the time only
	timestamp *time.Time `param:"timestamp,seconds"` // nolint:golint,structcheck

	// From field is a trade id, set ot return trades created after the trade
	from *int64 `param:"from"` // nolint:golint,structcheck

	// To field trade id, set to return trades created before the trade
	to *int64 `param:"to"` // nolint:golint,structcheck

	orderBy *string `param:"order_by"`

	pagination *bool `param:"pagination"`

	limit *int64 `param:"limit"`

	offset *int64 `param:"offset"`
}
