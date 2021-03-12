package ftx

import (
	"context"
	"encoding/json"
	"fmt"
)

type orderRequest struct {
	*restRequest
}

func (r *orderRequest) OpenOrders(ctx context.Context, market string) (orders, error) {
func (r *orderRequest) OpenOrders(ctx context.Context, market string) (ordersResponse, error) {
	resp, err := r.
		Method("GET").
		ReferenceURL("api/orders").
		Payloads(map[string]interface{}{"market": market}).
		DoAuthenticatedRequest(ctx)

	if err != nil {
		return ordersResponse{}, err
	}

	var o ordersResponse
	if err := json.Unmarshal(resp.Body, &o); err != nil {
		return ordersResponse{}, fmt.Errorf("failed to unmarshal open orders response body to json: %w", err)
	}

	return o, nil
}
