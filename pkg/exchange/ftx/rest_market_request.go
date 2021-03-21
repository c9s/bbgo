package ftx

import (
	"context"
	"encoding/json"
	"fmt"
)

type marketRequest struct {
	*restRequest
}

func (r *marketRequest) Markets(ctx context.Context) (marketsResponse, error) {
	resp, err := r.
		Method("GET").
		ReferenceURL("api/markets").
		DoAuthenticatedRequest(ctx)

	if err != nil {
		return marketsResponse{}, err
	}

	var m marketsResponse
	if err := json.Unmarshal(resp.Body, &m); err != nil {
		return marketsResponse{}, fmt.Errorf("failed to unmarshal market response body to json: %w", err)
	}

	return m, nil
}
