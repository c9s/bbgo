package coinbase

import (
	"context"

	"github.com/c9s/bbgo/pkg/types"
)

func (client *RestAPIClient) NewGetOrdersRequest() *GetOrdersRequest {
	req := GetOrdersRequest{
		client: client,
	}
	return &req
}

func (client *RestAPIClient) GetSingleOrder(ctx context.Context, orderID string) (*Order, error) {
	req := GetSingleOrderRequest{
		client:  client,
		orderID: orderID,
	}
	return req.Do(ctx)
}

func (client *RestAPIClient) DeleteOrder(ctx context.Context, orderIDs []string) {
	for _, orderID := range orderIDs {
		req := client.NewCancelOrderRequest(orderID)
		req.Do(ctx)
	}

}

func (client *RestAPIClient) CreateOrder(ctx context.Context, order types.SubmitOrder) (*CreateOrderResponse, error) {
	req := client.NewCreateOrderRequest(order)
	res, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}
	return res, nil
}
