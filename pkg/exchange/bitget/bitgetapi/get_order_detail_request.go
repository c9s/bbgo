package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type OrderDetail struct {
	AccountId        types.StrInt64             `json:"accountId"`
	Symbol           string                     `json:"symbol"`
	OrderId          types.StrInt64             `json:"orderId"`
	ClientOrderId    string                     `json:"clientOrderId"`
	Price            fixedpoint.Value           `json:"price"`
	Quantity         fixedpoint.Value           `json:"quantity"`
	OrderType        OrderType                  `json:"orderType"`
	Side             OrderSide                  `json:"side"`
	Status           OrderStatus                `json:"status"`
	FillPrice        fixedpoint.Value           `json:"fillPrice"`
	FillQuantity     fixedpoint.Value           `json:"fillQuantity"`
	FillTotalAmount  fixedpoint.Value           `json:"fillTotalAmount"`
	EnterPointSource string                     `json:"enterPointSource"`
	CTime            types.MillisecondTimestamp `json:"cTime"`
}

//go:generate PostRequest -url "/api/spot/v1/trade/orderInfo" -type GetOrderDetailRequest -responseDataType []OrderDetail
type GetOrderDetailRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol        string  `param:"symbol"`
	orderId       *string `param:"orderId"`
	clientOrderId *string `param:"clientOid"`
}

func (c *RestClient) NewGetOrderDetailRequest() *GetOrderDetailRequest {
	return &GetOrderDetailRequest{client: c}
}
