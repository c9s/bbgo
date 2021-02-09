package ftx

import (
	"context"
	"encoding/json"
	"fmt"
)

type balanceRequest struct {
	*restRequest
}

func (r *balanceRequest) Balances(ctx context.Context) (balances, error) {
	resp, err := r.
		Method("GET").
		ReferenceURL("api/wallet/balances").
		DoAuthenticatedRequest(ctx)

	if err != nil {
		return balances{}, err
	}

	var b balances
	if err := json.Unmarshal(resp.Body, &b); err != nil {
		return balances{}, fmt.Errorf("failed to unmarshal balance response body to json: %w", err)
	}

	return b, nil
}
