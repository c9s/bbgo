package okexapi

import "github.com/c9s/requestgen"

type TradeMode string

const (
	TradeModeCash     TradeMode = "cash"
	TradeModeIsolated TradeMode = "isolated"
	TradeModeCross    TradeMode = "cross"
)

type TargetCurrency string

const (
	TargetCurrencyBase  TargetCurrency = "base_ccy"
	TargetCurrencyQuote TargetCurrency = "quote_ccy"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type OrderResponse struct {
	OrderID       string `json:"ordId"`
	ClientOrderID string `json:"clOrdId"`
	Tag           string `json:"tag"`
	Code          string `json:"sCode"`
	Message       string `json:"sMsg"`
}

//go:generate PostRequest -url "/api/v5/trade/order" -type PlaceOrderRequest -responseDataType []OrderResponse
type PlaceOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	instrumentID string `param:"instId"`

	// tdMode
	// margin mode: "cross", "isolated"
	// non-margin mode cash
	tradeMode TradeMode `param:"tdMode" validValues:"cross,isolated,cash"`

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.
	clientOrderID *string `param:"clOrdId"`

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 8 characters.
	tag *string `param:"tag"`

	// "buy" or "sell"
	side SideType `param:"side" validValues:"buy,sell"`

	orderType OrderType `param:"ordType"`

	size string `param:"sz"`

	// price
	price *string `param:"px"`

	// Whether the target currency uses the quote or base currency.
	// base_ccy: Base currency ,quote_ccy: Quote currency
	// Only applicable to SPOT Market Orders
	// Default is quote_ccy for buy, base_ccy for sell
	targetCurrency *TargetCurrency `param:"tgtCcy" validValues:"quote_ccy,base_ccy"`
}

func (c *RestClient) NewPlaceOrderRequest() *PlaceOrderRequest {
	return &PlaceOrderRequest{
		client:    c,
		tradeMode: TradeModeCash,
	}
}
