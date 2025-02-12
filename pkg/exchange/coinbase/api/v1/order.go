package coinbase

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

func (client *RestAPIClient) GetOrders(ctx context.Context, symbol string, status []string, limit int,
	sortedBy *string, sorting *string, before *time.Time) (OrderSnapshot, error) {
	req := GetOrdersRequest{
		client:    client,
		productID: &symbol,
		status:    status,
		limit:     limit,
		sortedBy:  sortedBy,
		sorting:   sorting,
		before:    before,
	}
	return req.Do(ctx)
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
