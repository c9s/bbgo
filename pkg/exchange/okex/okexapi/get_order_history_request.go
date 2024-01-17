package okexapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data
type OrderDetail struct {
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
	TargetCurrency TargetCurrency             `json:"tgtCcy"`
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
	TradeMode       TradeMode        `json:"tdMode"`
	TpOrdPx         fixedpoint.Value `json:"tpOrdPx"`
	TpTriggerPx     fixedpoint.Value `json:"tpTriggerPx"`
	TpTriggerPxType string           `json:"tpTriggerPxType"`
	ReduceOnly      string           `json:"reduceOnly"`
	AlgoClOrdId     string           `json:"algoClOrdId"`
	AlgoId          string           `json:"algoId"`
}

//go:generate GetRequest -url "/api/v5/trade/orders-history-archive" -type GetOrderHistoryRequest -responseDataType []OrderDetail
type GetOrderHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	instrumentType InstrumentType `param:"instType,query"`
	instrumentID   *string        `param:"instId,query"`
	orderType      *OrderType     `param:"ordType,query"`
	// underlying and instrumentFamil Applicable to FUTURES/SWAP/OPTION
	underlying       *string `param:"uly,query"`
	instrumentFamily *string `param:"instFamily,query"`

	state     *OrderState `param:"state,query"`
	after     *string     `param:"after,query"`
	before    *string     `param:"before,query"`
	startTime *time.Time  `param:"begin,query,milliseconds"`

	// endTime for each request, startTime and endTime can be any interval, but should be in last 3 months
	endTime *time.Time `param:"end,query,milliseconds"`

	// limit for data size per page. Default: 100
	limit *uint64 `param:"limit,query"`
}

// NewGetOrderHistoriesRequest is descending order by createdTime
func (c *RestClient) NewGetOrderHistoryRequest() *GetOrderHistoryRequest {
	return &GetOrderHistoryRequest{
		client:         c,
		instrumentType: InstrumentTypeSpot,
	}
}
