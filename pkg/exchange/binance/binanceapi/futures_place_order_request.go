package binanceapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type PositionSide string

const (
	PositionSideLong  PositionSide = "LONG"
	PositionSideShort PositionSide = "SHORT"
	PositionSideBoth  PositionSide = "BOTH"
)

type FuturesOrderType string

const (
	FuturesOrderTypeLimit              FuturesOrderType = "LIMIT"
	FuturesOrderTypeMarket             FuturesOrderType = "MARKET"
	FuturesOrderTypeStop               FuturesOrderType = "STOP"
	FuturesOrderTypeStopMarket         FuturesOrderType = "STOP_MARKET"
	FuturesOrderTypeTakeProfit         FuturesOrderType = "TAKE_PROFIT"
	FuturesOrderTypeTakeProfitMarket   FuturesOrderType = "TAKE_PROFIT_MARKET"
	FuturesOrderTypeTrailingStopMarket FuturesOrderType = "TRAILING_STOP_MARKET"
)

type FuturesOrderResponse struct {
	ClientOrderId           string           `json:"clientOrderId"`
	CumQty                  fixedpoint.Value `json:"cumQty"`
	CumQuote                fixedpoint.Value `json:"cumQuote"`
	ExecutedQty             fixedpoint.Value `json:"executedQty"`
	OrderId                 int              `json:"orderId"`
	AvgPrice                fixedpoint.Value `json:"avgPrice"`
	OrigQty                 fixedpoint.Value `json:"origQty"`
	Price                   fixedpoint.Value `json:"price"`
	ReduceOnly              bool             `json:"reduceOnly"`
	Side                    SideType         `json:"side"`
	PositionSide            PositionSide     `json:"positionSide"`
	Status                  string           `json:"status"`
	StopPrice               string           `json:"stopPrice"`
	ClosePosition           bool             `json:"closePosition"`
	Symbol                  string           `json:"symbol"`
	TimeInForce             string           `json:"timeInForce"`
	Type                    OrderType        `json:"type"`
	OrigType                string           `json:"origType"`
	ActivatePrice           string           `json:"activatePrice"`
	PriceRate               string           `json:"priceRate"`
	UpdateTime              int64            `json:"updateTime"`
	WorkingType             string           `json:"workingType"`
	PriceProtect            bool             `json:"priceProtect"`
	PriceMatch              string           `json:"priceMatch"`
	SelfTradePreventionMode string           `json:"selfTradePreventionMode"`
	GoodTillDate            int64            `json:"goodTillDate"`
}

//go:generate requestgen -method POST -url "/fapi/v1/order" -type FuturesPlaceOrderRequest -responseType []FuturesPositionRisk
type FuturesPlaceOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol       string       `param:"symbol"`
	side         SideType     `param:"side"`
	positionSide PositionSide `param:"positionSide"`

	quantity        *fixedpoint.Value `param:"quantity"`
	price           *fixedpoint.Value `param:"price"`
	stopPrice       *fixedpoint.Value `param:"stopPrice"`
	activationPrice *fixedpoint.Value `param:"activationPrice"`
	callbackRate    *fixedpoint.Value `param:"callbackRate"`
	priceProtect    *string           `param:"priceProtect"` // "TRUE" or "FALSE"
}

func (c *FuturesRestClient) NewFuturesPlaceOrderRequest() *FuturesPlaceOrderRequest {
	return &FuturesPlaceOrderRequest{client: c}
}
