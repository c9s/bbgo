package ftx

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type orderRequest struct {
	*restRequest
}

/*
{
  "market": "XRP-PERP",
  "side": "sell",
  "price": 0.306525,
  "type": "limit",
  "size": 31431.0,
  "reduceOnly": false,
  "ioc": false,
  "postOnly": false,
  "clientId": null
}
*/
type PlaceOrderPayload struct {
	Market     string
	Side       string
	Price      fixedpoint.Value
	Type       string
	Size       fixedpoint.Value
	ReduceOnly bool
	IOC        bool
	PostOnly   bool
	ClientID   string
}

