package bitgetapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type TradeScope string

const (
	TradeMaker TradeScope = "maker"
	TradeTaker TradeScope = "taker"
)

type DiscountStatus string

const (
	DiscountYes DiscountStatus = "yes"
	DiscountNo  DiscountStatus = "no"
)

type TradeFee struct {
	// Discount or not
	Deduction DiscountStatus `json:"deduction"`
	// Transaction fee coin
	FeeCoin string `json:"feeCoin"`
	// Total transaction fee discount
	TotalDeductionFee fixedpoint.Value `json:"totalDeductionFee"`
	// Total transaction fee
	TotalFee fixedpoint.Value `json:"totalFee"`
}

type Trade struct {
	UserId      types.StrInt64             `json:"userId"`
	Symbol      string                     `json:"symbol"`
	OrderId     types.StrInt64             `json:"orderId"`
	TradeId     types.StrInt64             `json:"tradeId"`
	OrderType   OrderType                  `json:"orderType"`
	Side        SideType                   `json:"side"`
	PriceAvg    fixedpoint.Value           `json:"priceAvg"`
	Size        fixedpoint.Value           `json:"size"`
	Amount      fixedpoint.Value           `json:"amount"`
	FeeDetail   TradeFee                   `json:"feeDetail"`
	TradeScope  TradeScope                 `json:"tradeScope"`
	CreatedTime types.MillisecondTimestamp `json:"cTime"`
	UpdatedTime types.MillisecondTimestamp `json:"uTime"`
}

//go:generate GetRequest -url "/api/v2/spot/trade/fills" -type GetTradeFillsRequest -responseDataType []Trade
type GetTradeFillsRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol,query"`
	// Limit number default 100 max 100
	limit *string `param:"limit,query"`
	// idLessThan requests the content on the page before this ID (older data), the value input should be the orderId of the corresponding interface.
	idLessThan *string    `param:"idLessThan,query"`
	startTime  *time.Time `param:"startTime,milliseconds,query"`
	endTime    *time.Time `param:"endTime,milliseconds,query"`
	orderId    *string    `param:"orderId,query"`
}

func (s *Client) NewGetTradeFillsRequest() *GetTradeFillsRequest {
	return &GetTradeFillsRequest{client: s.Client}
}
