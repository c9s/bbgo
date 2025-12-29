package binanceapi

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/requestgen"
)

type AlgoType string

const (
	AlgoTypeConditional AlgoType = "CONDITIONAL"
)

type FuturesAlgoOrderType string

const (
	FuturesAlgoOrderTypeStopMarket         FuturesAlgoOrderType = "STOP_MARKET"
	FuturesAlgoOrderTypeTakeProfitMarket   FuturesAlgoOrderType = "TAKE_PROFIT_MARKET"
	FuturesAlgoOrderTypeStop               FuturesAlgoOrderType = "STOP"
	FuturesAlgoOrderTypeTakeProfit         FuturesAlgoOrderType = "TAKE_PROFIT"
	FuturesAlgoOrderTypeTrailingStopMarket FuturesAlgoOrderType = "TRAILING_STOP_MARKET"
)

type WorkingType string

const (
	WorkingTypeMarkPrice     WorkingType = "MARK_PRICE"
	WorkingTypeContractPrice WorkingType = "CONTRACT_PRICE"
)

type FuturesPlaceAlgoOrderResponse struct {
	AlgoId                  int64             `json:"algoId"`
	ClientAlgoId            string            `json:"clientAlgoId"`
	AlgoType                string            `json:"algoType"`
	OrderType               string            `json:"orderType"`
	Symbol                  string            `json:"symbol"`
	Side                    SideType          `json:"side"`
	PositionSide            PositionSide      `json:"positionSide"`
	TimeInForce             string            `json:"timeInForce"`
	Quantity                fixedpoint.Value  `json:"quantity"`
	AlgoStatus              string            `json:"algoStatus"`
	TriggerPrice            fixedpoint.Value  `json:"triggerPrice"`
	Price                   fixedpoint.Value  `json:"price"`
	IcebergQuantity         *fixedpoint.Value `json:"icebergQuantity"`
	SelfTradePreventionMode string            `json:"selfTradePreventionMode"`
	WorkingType             string            `json:"workingType"`
	PriceMatch              string            `json:"priceMatch"`
	ClosePosition           bool              `json:"closePosition"`
	PriceProtect            bool              `json:"priceProtect"`
	ReduceOnly              bool              `json:"reduceOnly"`
	ActivatePrice           string            `json:"activatePrice"`
	CallbackRate            string            `json:"callbackRate"`
	CreateTime              int64             `json:"createTime"`
	UpdateTime              int64             `json:"updateTime"`
	TriggerTime             int64             `json:"triggerTime"`
	GoodTillDate            int64             `json:"goodTillDate"`
}

//go:generate requestgen -method POST -url "/fapi/v1/algoOrder" -type FuturesPlaceAlgoOrderRequest -responseType .FuturesPlaceAlgoOrderResponse
type FuturesPlaceAlgoOrderRequest struct {
	client requestgen.AuthenticatedAPIClient

	// Required parameters
	algoType  AlgoType             `param:"algoType"`
	symbol    string               `param:"symbol"`
	side      SideType             `param:"side"`
	orderType FuturesAlgoOrderType `param:"type"`

	// Optional parameters
	positionSide            *PositionSide                `param:"positionSide"`
	timeInForce             *TimeInForceType             `param:"timeInForce"`
	quantity                *string                      `param:"quantity"`
	price                   *string                      `param:"price"`
	triggerPrice            *string                      `param:"triggerPrice"`
	workingType             *WorkingType                 `param:"workingType"`
	priceMatch              *PriceMatchType              `param:"priceMatch"`
	closePosition           *bool                        `param:"closePosition"` // "true" or "false"
	priceProtect            *string                      `param:"priceProtect"`  // "TRUE" or "FALSE"
	reduceOnly              *bool                        `param:"reduceOnly"`    // "true" or "false"
	activatePrice           *string                      `param:"activatePrice"`
	callbackRate            *string                      `param:"callbackRate"`
	clientAlgoId            *string                      `param:"clientAlgoId"`
	selfTradePreventionMode *SelfTradePreventionModeType `param:"selfTradePreventionMode"`
	goodTillDate            *int64                       `param:"goodTillDate"`
}

func (c *FuturesRestClient) NewFuturesPlaceAlgoOrderRequest() *FuturesPlaceAlgoOrderRequest {
	return &FuturesPlaceAlgoOrderRequest{
		client:   c,
		algoType: AlgoTypeConditional, // Default to CONDITIONAL as it's the only supported type
	}
}
