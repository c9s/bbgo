package okexapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type OpenOrder struct {
	AccumulatedFillSize fixedpoint.Value `json:"accFillSz"`
	// If none is filled, it will return "".
	AvgPrice      fixedpoint.Value           `json:"avgPx"`
	CreatedTime   types.MillisecondTimestamp `json:"cTime"`
	Category      string                     `json:"category"`
	ClientOrderId string                     `json:"clOrdId"`
	Fee           fixedpoint.Value           `json:"fee"`
	FeeCurrency   string                     `json:"feeCcy"`
	// Last filled time
	FillTime       types.MillisecondTimestamp `json:"fillTime"`
	InstrumentID   string                     `json:"instId"`
	InstrumentType InstrumentType             `json:"instType"`
	OrderId        types.StrInt64             `json:"ordId"`
	OrderType      OrderType                  `json:"ordType"`
	Price          fixedpoint.Value           `json:"px"`
	Side           SideType                   `json:"side"`
	State          OrderState                 `json:"state"`
	Size           fixedpoint.Value           `json:"sz"`
	TargetCurrency string                     `json:"tgtCcy"`
	UpdatedTime    types.MillisecondTimestamp `json:"uTime"`

	// Margin currency
	// Only applicable to cross MARGIN orders in Single-currency margin.
	Currency string `json:"ccy"`
	TradeId  string `json:"tradeId"`
	// Last filled price
	FillPrice fixedpoint.Value `json:"fillPx"`
	// Last filled quantity
	FillSize fixedpoint.Value `json:"fillSz"`
	// Leverage, from 0.01 to 125.
	// Only applicable to MARGIN/FUTURES/SWAP
	Lever string `json:"lever"`
	// Profit and loss, Applicable to orders which have a trade and aim to close position. It always is 0 in other conditions
	Pnl          fixedpoint.Value `json:"pnl"`
	PositionSide string           `json:"posSide"`
	// Options price in USDOnly applicable to options; return "" for other instrument types
	PriceUsd fixedpoint.Value `json:"pxUsd"`
	// Implied volatility of the options orderOnly applicable to options; return "" for other instrument types
	PriceVol fixedpoint.Value `json:"pxVol"`
	// Price type of options
	PriceType string `json:"pxType"`
	// Rebate amount, only applicable to spot and margin, the reward of placing orders from the platform (rebate)
	// given to user who has reached the specified trading level. If there is no rebate, this field is "".
	Rebate    fixedpoint.Value `json:"rebate"`
	RebateCcy string           `json:"rebateCcy"`
	// Client-supplied Algo ID when placing order attaching TP/SL.
	AttachAlgoClOrdId string           `json:"attachAlgoClOrdId"`
	SlOrdPx           fixedpoint.Value `json:"slOrdPx"`
	SlTriggerPx       fixedpoint.Value `json:"slTriggerPx"`
	SlTriggerPxType   string           `json:"slTriggerPxType"`
	AttachAlgoOrds    []interface{}    `json:"attachAlgoOrds"`
	Source            string           `json:"source"`
	// Self trade prevention ID. Return "" if self trade prevention is not applicable
	StpId string `json:"stpId"`
	// Self trade prevention mode. Return "" if self trade prevention is not applicable
	StpMode         string           `json:"stpMode"`
	Tag             string           `json:"tag"`
	TradeMode       string           `json:"tdMode"`
	TpOrdPx         fixedpoint.Value `json:"tpOrdPx"`
	TpTriggerPx     fixedpoint.Value `json:"tpTriggerPx"`
	TpTriggerPxType string           `json:"tpTriggerPxType"`
	ReduceOnly      string           `json:"reduceOnly"`
	QuickMgnType    string           `json:"quickMgnType"`
	AlgoClOrdId     string           `json:"algoClOrdId"`
	AlgoId          string           `json:"algoId"`
}

//go:generate GetRequest -url "/api/v5/trade/orders-pending" -type GetOpenOrdersRequest -responseDataType []OpenOrder
type GetOpenOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	instrumentID *string `param:"instId,query"`

	instrumentType InstrumentType `param:"instType,query"`

	orderType *OrderType `param:"ordType,query"`

	state    *OrderState `param:"state,query"`
	category *string     `param:"category,query"`
	// Pagination of data to return records earlier than the requested ordId
	after *string `param:"after,query"`
	// Pagination of data to return records newer than the requested ordId
	before *string `param:"before,query"`
	// Filter with a begin timestamp. Unix timestamp format in milliseconds, e.g. 1597026383085
	begin *time.Time `param:"begin,query"`

	// Filter with an end timestamp. Unix timestamp format in milliseconds, e.g. 1597026383085
	end   *time.Time `param:"end,query"`
	limit *string    `param:"limit,query"`
}

func (c *RestClient) NewGetOpenOrdersRequest() *GetOpenOrdersRequest {
	return &GetOpenOrdersRequest{
		client:         c,
		instrumentType: InstrumentTypeSpot,
	}
}
