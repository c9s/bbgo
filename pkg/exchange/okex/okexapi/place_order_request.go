package okexapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/types/strint"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type OrderResponse struct {
	OrderID       string `json:"ordId"`
	ClientOrderID string `json:"clOrdId"`
	Tag           string `json:"tag"`

	Timestamp strint.Int64 `json:"ts"`
	Code      string       `json:"sCode"`
	Message   string       `json:"sMsg"`
}

//go:generate PostRequest -url "/api/v5/trade/order" -type PlaceOrderRequest -responseDataType []OrderResponse
type PlaceOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	instrumentID string `param:"instId"`

	// tdMode
	// margin mode: "cross", "isolated"
	// non-margin mode cash
	tradeMode TradeMode `param:"tdMode" validValues:"cross,isolated,cash"`

	// clientOrderID is a combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.
	clientOrderID *string `param:"clOrdId"`

	// tag is a combination of case-sensitive alphanumerics, all numbers, or all letters of up to 16 characters.
	tag *string `param:"tag"`

	// currency is the margin currency
	// Only applicable to cross MARGIN orders in Spot and futures mode.
	currency *string `param:"ccy"`

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

	// reduceOnly -- whether orders can only reduce in position size.
	// Valid options: true or false. The default value is false.
	// Only applicable to MARGIN orders, and FUTURES/SWAP orders in net mode
	// Only applicable to Spot and futures mode and Multi-currency margin
	reduceOnly *bool `param:"reduceOnly"`

	// Position side
	// The default is net in the net mode
	// It is required in the long/short mode, and can only be long or short.
	// Only applicable to FUTURES/SWAP.
	posSide *PosSide `param:"posSide" validValues:"long,short"`
}

func (c *RestClient) NewPlaceOrderRequest() *PlaceOrderRequest {
	return &PlaceOrderRequest{
		client:    c,
		tradeMode: TradeModeCash,
	}
}
