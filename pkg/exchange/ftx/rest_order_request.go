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
	resp, err := r.
		Method("GET").
		ReferenceURL("api/orders").
		Payloads(map[string]interface{}{"market": market}).
		DoAuthenticatedRequest(ctx)

	if err != nil {
		return orders{}, err
	}

	var o orders
	if err := json.Unmarshal(resp.Body, &o); err != nil {
		return orders{}, fmt.Errorf("failed to unmarshal open orders response body to json: %w", err)
	}

	return o, nil
}
