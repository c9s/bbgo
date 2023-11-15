package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

type FeeDetail struct {
	// NewFees should have a value because when I was integrating, it already prompted,
	// "If there is no 'newFees' field, this data represents earlier historical data."
	NewFees struct {
		// Amount deducted by coupons, unit：currency obtained from the transaction.
		DeductedByCoupon fixedpoint.Value `json:"c"`
		// Amount deducted in BGB (Bitget Coin), unit：BGB
		DeductedInBGB fixedpoint.Value `json:"d"`
		// If the BGB balance is insufficient to cover the fees, the remaining amount is deducted from the
		//currency obtained from the transaction.
		DeductedFromCurrency fixedpoint.Value `json:"r"`
		// The total fee amount to be paid, unit ：currency obtained from the transaction.
		ToBePaid fixedpoint.Value `json:"t"`
		// ignored
		Deduction bool `json:"deduction"`
		// ignored
		TotalDeductionFee fixedpoint.Value `json:"totalDeductionFee"`
	} `json:"newFees"`
}

type OrderDetail struct {
	UserId types.StrInt64 `json:"userId"`
	Symbol string         `json:"symbol"`
	// OrderId are always numeric. It's confirmed with official customer service. https://t.me/bitgetOpenapi/24172
	OrderId       types.StrInt64   `json:"orderId"`
	ClientOrderId string           `json:"clientOid"`
	Price         fixedpoint.Value `json:"price"`
	// Size is base coin when orderType=limit; quote coin when orderType=market
	Size             fixedpoint.Value `json:"size"`
	OrderType        OrderType        `json:"orderType"`
	Side             SideType         `json:"side"`
	Status           OrderStatus      `json:"status"`
	PriceAvg         fixedpoint.Value `json:"priceAvg"`
	BaseVolume       fixedpoint.Value `json:"baseVolume"`
	QuoteVolume      fixedpoint.Value `json:"quoteVolume"`
	EnterPointSource string           `json:"enterPointSource"`
	// The value is json string, so we unmarshal it after unmarshal OrderDetail
	FeeDetailRaw string                     `json:"feeDetail"`
	OrderSource  string                     `json:"orderSource"`
	CreatedTime  types.MillisecondTimestamp `json:"cTime"`
	UpdatedTime  types.MillisecondTimestamp `json:"uTime"`

	FeeDetail FeeDetail
}

func (o *OrderDetail) UnmarshalJSON(data []byte) error {
	if o == nil {
		return fmt.Errorf("failed to unmarshal json from nil pointer order detail")
	}
	// define new type to avoid loop reference
	type AuxOrderDetail OrderDetail

	var aux AuxOrderDetail
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*o = OrderDetail(aux)

	if len(aux.FeeDetailRaw) == 0 {
		return nil
	}

	var feeDetail FeeDetail
	if err := json.Unmarshal([]byte(aux.FeeDetailRaw), &feeDetail); err != nil {
		return fmt.Errorf("unexpected fee detail raw: %s, err: %w", aux.FeeDetailRaw, err)
	}
	o.FeeDetail = feeDetail

	return nil
}

//go:generate GetRequest -url "/api/v2/spot/trade/history-orders" -type GetHistoryOrdersRequest -responseDataType []OrderDetail
type GetHistoryOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol *string `param:"symbol,query"`
	// Limit number default 100 max 100
	limit *string `param:"limit,query"`
	// idLessThan requests the content on the page before this ID (older data), the value input should be the orderId of the corresponding interface.
	idLessThan *string    `param:"idLessThan,query"`
	startTime  *time.Time `param:"startTime,milliseconds,query"`
	endTime    *time.Time `param:"endTime,milliseconds,query"`
	orderId    *string    `param:"orderId,query"`
}

func (c *Client) NewGetHistoryOrdersRequest() *GetHistoryOrdersRequest {
	return &GetHistoryOrdersRequest{client: c.Client}
}
