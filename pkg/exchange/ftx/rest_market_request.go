package ftx

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

type marketRequest struct {
	*restRequest
}

/*
supported resolutions: window length in seconds. options: 15, 60, 300, 900, 3600, 14400, 86400
doc: https://docs.ftx.com/?javascript#get-historical-prices
*/
func (r *marketRequest) HistoricalPrices(ctx context.Context, market string, interval types.Interval, limit int64, start, end *time.Time) (HistoricalPricesResponse, error) {
	q := map[string]string{
		"resolution": strconv.FormatInt(int64(interval.Minutes())*60, 10),
	}

	if limit > 0 {
		q["limit"] = strconv.FormatInt(limit, 10)
	}

	if start != nil {
		q["start_time"] = strconv.FormatInt(start.Unix(), 10)
	}

	if end != nil {
		q["end_time"] = strconv.FormatInt(end.Unix(), 10)
	}

	resp, err := r.
		Method("GET").
		Query(q).
		ReferenceURL(fmt.Sprintf("api/markets/%s/candles", market)).
		DoAuthenticatedRequest(ctx)

	if err != nil {
		return HistoricalPricesResponse{}, err
	}

	var h HistoricalPricesResponse
	if err := json.Unmarshal(resp.Body, &h); err != nil {
		return HistoricalPricesResponse{}, fmt.Errorf("failed to unmarshal historical prices response body to json: %w", err)
	}
	return h, nil
}
