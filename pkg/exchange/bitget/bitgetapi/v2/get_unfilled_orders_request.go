package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type UnfilledOrder struct {
	UserId types.StrInt64 `json:"userId"`
	Symbol string         `json:"symbol"`
	// OrderId are always numeric. It's confirmed with official customer service. https://t.me/bitgetOpenapi/24172
	OrderId       types.StrInt64   `json:"orderId"`
	ClientOrderId string           `json:"clientOid"`
	PriceAvg      fixedpoint.Value `json:"priceAvg"`
	// Size is base coin when orderType=limit; quote coin when orderType=market
	Size             fixedpoint.Value           `json:"size"`
	OrderType        OrderType                  `json:"orderType"`
	Side             SideType                   `json:"side"`
	Status           OrderStatus                `json:"status"`
	BasePrice        fixedpoint.Value           `json:"basePrice"`
	BaseVolume       fixedpoint.Value           `json:"baseVolume"`
	QuoteVolume      fixedpoint.Value           `json:"quoteVolume"`
	EnterPointSource string                     `json:"enterPointSource"`
	OrderSource      string                     `json:"orderSource"`
	CreatedTime      types.MillisecondTimestamp `json:"cTime"`
	UpdatedTime      types.MillisecondTimestamp `json:"uTime"`
}

//go:generate GetRequest -url "/api/v2/spot/trade/unfilled-orders" -type GetUnfilledOrdersRequest -responseDataType []UnfilledOrder
type GetUnfilledOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol *string `param:"symbol,query"`

	// limit number default 100 max 100
	limit *string `param:"limit,query"`

	// idLessThan requests the content on the page before this ID (older data), the value input should be the orderId of the corresponding interface.
	idLessThan *string `param:"idLessThan,query"`

	startTime *time.Time `param:"startTime,milliseconds,query"`
	endTime   *time.Time `param:"endTime,milliseconds,query"`

	orderId *string `param:"orderId,query"`
}

func (c *Client) NewGetUnfilledOrdersRequest() *GetUnfilledOrdersRequest {
	return &GetUnfilledOrdersRequest{client: c.Client}
}
