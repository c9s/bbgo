package ftx

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type fillsRequest struct {
	*restRequest
}

func (r *fillsRequest) Fills(ctx context.Context, market string, since, until time.Time, limit int64, orderByASC bool) (fillsResponse, error) {
	q := make(map[string]string)
	if len(market) > 0 {
		q["market"] = market
	}
	if since != (time.Time{}) {
		q["start_time"] = strconv.FormatInt(since.Unix(), 10)
	}
	if until != (time.Time{}) {
		q["end_time"] = strconv.FormatInt(until.Unix(), 10)
	}
	if limit > 0 {
		q["limit"] = strconv.FormatInt(limit, 10)
	}
	// default is descending
	if orderByASC {
		q["order"] = "asc"
	}
	resp, err := r.
		Method("GET").
		ReferenceURL("api/fills").
		Query(q).
		DoAuthenticatedRequest(ctx)

	if err != nil {
		return fillsResponse{}, err
	}

	var f fillsResponse
	if err := json.Unmarshal(resp.Body, &f); err != nil {
		fmt.Println("??? => ", resp.Body)
		return fillsResponse{}, fmt.Errorf("failed to unmarshal fills response body to json: %w", err)
	}

	return f, nil
}
