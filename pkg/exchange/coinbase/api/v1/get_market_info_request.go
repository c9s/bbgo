package coinbase

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/requestgen"
)

type MarketInfo struct {
	ID                     string           `json:"id"`
	BaseCurrency           string           `json:"base_currency"`
	QuoteCurrency          string           `json:"quote_currency"`
	QuoteIncrement         fixedpoint.Value `json:"quote_increment"`
	BaseIncrement          fixedpoint.Value `json:"base_increment"`
	DisplayName            string           `json:"display_name"`
	MinMarketFunds         fixedpoint.Value `json:"min_market_funds"`
	MarginEnabled          bool             `json:"margin_enabled"`
	PostOnly               bool             `json:"post_only"`
	LimitOnly              bool             `json:"limit_only"`
	CancelOnly             bool             `json:"cancel_only"`
	Status                 MarketStatus     `json:"status"`
	StatusMessage          string           `json:"status_message"`
	AuctionMode            bool             `json:"auction_mode"`
	TradingDisabled        bool             `json:"trading_disabled,omitempty"`
	FxStablecoin           bool             `json:"fx_stablecoin,omitempty"`
	MaxSlippagePercentage  string           `json:"max_slippage_percentage,omitempty"`
	HighBidLimitPercentage string           `json:"high_bid_limit_percentage,omitempty"`
}

type MarketInfoResponse []MarketInfo

// https://docs.cdp.coinbase.com/exchange/reference/exchangerestapi_getproducts
//
//go:generate requestgen -method GET -url /products -rateLimiter 1+20/2s -type GetMarketInfoRequest -responseType .MarketInfoResponse
type GetMarketInfoRequest struct {
	client requestgen.APIClient
}

func (c *RestAPIClient) NewGetMarketInfoRequest() *GetMarketInfoRequest {
	return &GetMarketInfoRequest{client: c}
}
