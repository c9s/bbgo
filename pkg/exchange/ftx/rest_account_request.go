package ftx

import (
	"context"
	"encoding/json"
	"fmt"
)

type accountRequest struct {
	*restRequest
}

func (r *accountRequest) Account(ctx context.Context) (accountResponse, error) {
	resp, err := r.
		Method("GET").
		ReferenceURL("api/account").
		DoAuthenticatedRequest(ctx)

	if err != nil {
		return accountResponse{}, err
	}

	var a accountResponse
	if err := json.Unmarshal(resp.Body, &a); err != nil {
		return accountResponse{}, fmt.Errorf("failed to unmarshal account response body to json: %w", err)
	}

	return a, nil
}

func (r *accountRequest) Positions(ctx context.Context) (positionsResponse, error) {
	resp, err := r.
		Method("GET").
		ReferenceURL("api/positions").
		DoAuthenticatedRequest(ctx)

	if err != nil {
		return positionsResponse{}, err
	}

	var p positionsResponse
	if err := json.Unmarshal(resp.Body, &p); err != nil {
		return positionsResponse{}, fmt.Errorf("failed to unmarshal position response body to json: %w", err)
	}

	return p, nil
}
