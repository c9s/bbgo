package ftx

import (
	"context"
	"encoding/json"
	"fmt"
)

type walletRequest struct {
	*restRequest
}

func (r *walletRequest) DepositHistory(ctx context.Context) (depositHistoryResponse, error) {
	resp, err := r.
		Method("GET").
		ReferenceURL("api/wallet/deposits").
		DoAuthenticatedRequest(ctx)

	if err != nil {
		return depositHistoryResponse{}, err
	}

	var d depositHistoryResponse
	if err := json.Unmarshal(resp.Body, &d); err != nil {
		return depositHistoryResponse{}, fmt.Errorf("failed to unmarshal deposit history response body to json: %w", err)
	}

	return d, nil
}

func (r *walletRequest) Balances(ctx context.Context) (balances, error) {
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
