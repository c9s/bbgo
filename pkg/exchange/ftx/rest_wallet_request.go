package ftx

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type walletRequest struct {
	*restRequest
}

func (r *walletRequest) DepositHistory(ctx context.Context, since time.Time, until time.Time, limit int) (depositHistoryResponse, error) {
	q := make(map[string]string)
	if limit > 0 {
		q["limit"] = strconv.Itoa(limit)
	}

	if since != (time.Time{}) {
		q["start_time"] = strconv.FormatInt(since.Unix(), 10)
	}
	if until != (time.Time{}) {
		q["end_time"] = strconv.FormatInt(until.Unix(), 10)
	}

	resp, err := r.
		Method("GET").
		ReferenceURL("api/wallet/deposits").
		Query(q).
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
