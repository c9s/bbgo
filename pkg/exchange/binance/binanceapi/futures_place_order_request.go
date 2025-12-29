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
	FuturesOrderTypeLimit  FuturesOrderType = "LIMIT"
	FuturesOrderTypeMarket FuturesOrderType = "MARKET"
)

type TimeInForceType string

const (
	TimeInForceTypeGTC TimeInForceType = "GTC" // Good Till Cancel
	TimeInForceTypeIOC TimeInForceType = "IOC" // Immediate or Cancel
	TimeInForceTypeFOK TimeInForceType = "FOK" // Fill or Kill
	TimeInForceTypeGTD TimeInForceType = "GTD" // Good Till Date
)

type PriceMatchType string

const (
	PriceMatchNone       PriceMatchType = "NONE"
	PriceMatchOpponent   PriceMatchType = "OPPONENT"
	PriceMatchOpponent5  PriceMatchType = "OPPONENT_5"
	PriceMatchOpponent10 PriceMatchType = "OPPONENT_10"
	PriceMatchOpponent20 PriceMatchType = "OPPONENT_20"
	PriceMatchQueue      PriceMatchType = "QUEUE"
	PriceMatchQueue5     PriceMatchType = "QUEUE_5"
	PriceMatchQueue10    PriceMatchType = "QUEUE_10"
	PriceMatchQueue20    PriceMatchType = "QUEUE_20"
)

type SelfTradePreventionModeType string

const (
	SelfTradePreventionModeExpireTaker SelfTradePreventionModeType = "EXPIRE_TAKER"
	SelfTradePreventionModeExpireMaker SelfTradePreventionModeType = "EXPIRE_MAKER"
	SelfTradePreventionModeExpireBoth  SelfTradePreventionModeType = "EXPIRE_BOTH"
	SelfTradePreventionModeNone        SelfTradePreventionModeType = "NONE"
)

type FuturesOrderResponse struct {
	ClientOrderId           string           `json:"clientOrderId"`
	CumQty                  fixedpoint.Value `json:"cumQty"`
	CumQuote                fixedpoint.Value `json:"cumQuote"`
	ExecutedQty             fixedpoint.Value `json:"executedQty"`
	OrderId                 int64            `json:"orderId"`
	AvgPrice                fixedpoint.Value `json:"avgPrice"`
	OrigQty                 fixedpoint.Value `json:"origQty"`
	Price                   fixedpoint.Value `json:"price"`
	ReduceOnly              bool             `json:"reduceOnly"`
	Side                    SideType         `json:"side"`
	PositionSide            PositionSide     `json:"positionSide"`
	Status                  string           `json:"status"`
	StopPrice               fixedpoint.Value `json:"stopPrice"`
	ClosePosition           bool             `json:"closePosition"`
	Symbol                  string           `json:"symbol"`
	TimeInForce             string           `json:"timeInForce"`
	OrderType               FuturesOrderType `json:"type"`
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

//go:generate requestgen -method POST -url "/fapi/v1/order" -type FuturesPlaceOrderRequest -responseType .FuturesOrderResponse
type FuturesPlaceOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol       string           `param:"symbol"`
	side         SideType         `param:"side"`
	positionSide *PositionSide    `param:"positionSide"`
	orderType    FuturesOrderType `param:"type"`

	timeInForce             *TimeInForceType             `param:"timeInForce"`
	quantity                *string                      `param:"quantity"`
	reduceOnly              *bool                        `param:"reduceOnly"` // "true" or "false"
	price                   *string                      `param:"price"`
	newClientOrderId        *string                      `param:"newClientOrderId"`
	newOrderRespType        *OrderRespType               `param:"newOrderRespType"`
	priceMatch              *PriceMatchType              `param:"priceMatch"`
	selfTradePreventionMode *SelfTradePreventionModeType `param:"selfTradePreventionMode"`
	goodTillDate            *int64                       `param:"goodTillDate"`
}

func (c *FuturesRestClient) NewFuturesPlaceOrderRequest() *FuturesPlaceOrderRequest {
	return &FuturesPlaceOrderRequest{client: c}
}
