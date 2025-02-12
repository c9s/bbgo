package coinbase

import "context"

func (client *RestAPIClient) GetOrderTrades(ctx context.Context, orderID string, limit int, before *string) (TradeSnapshot, error) {
	req := GetOrderTradesRequest{
		client:  client,
		orderID: orderID,
		limit:   limit,
		before:  before,
	}
	return req.Do(ctx)
}
