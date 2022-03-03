package ftx

import (
	"context"
	"encoding/json"
	"fmt"

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

func (r *orderRequest) PlaceOrder(ctx context.Context, p PlaceOrderPayload) (orderResponse, error) {
	resp, err := r.
		Method("POST").
		ReferenceURL("api/orders").
		Payloads(map[string]interface{}{
			"market":     p.Market,
			"side":       p.Side,
			"price":      p.Price,
			"type":       p.Type,
			"size":       p.Size,
			"reduceOnly": p.ReduceOnly,
			"ioc":        p.IOC,
			"postOnly":   p.PostOnly,
			"clientId":   p.ClientID,
		}).
		DoAuthenticatedRequest(ctx)

	if err != nil {
		return orderResponse{}, err
	}
	var o orderResponse
	if err := json.Unmarshal(resp.Body, &o); err != nil {
		return orderResponse{}, fmt.Errorf("failed to unmarshal order response body to json: %w", err)
	}

	return o, nil
}
