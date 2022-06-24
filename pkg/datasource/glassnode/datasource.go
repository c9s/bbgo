package glassnode

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/datasource/glassnode/glassnodeapi"
)

type DataSource struct {
	client *glassnodeapi.RestClient
}

func New(apiKey string) *DataSource {
	client := glassnodeapi.NewRestClient()
	client.Auth(apiKey)

	return &DataSource{client: client}
}

func (d *DataSource) Query(ctx context.Context, category, metric, asset, interval string, since, until *time.Time) (glassnodeapi.DataSlice, error) {
	req := glassnodeapi.Request{
		Client: d.client,
		Asset:  asset,
		Format: glassnodeapi.FormatJSON,

		Category: category,
		Metric:   metric,
	}

	if since != nil {
		req.Since = since
	}

	if until != nil {
		req.Until = until
	}

	if interval != "" {
		req.SetInterval(glassnodeapi.Interval(interval))
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return glassnodeapi.DataSlice(resp), nil
}

// query last futures open interest
// https://docs.glassnode.com/api/derivatives#futures-open-interest
func (d *DataSource) QueryFuturesOpenInterest(ctx context.Context, currency string) (float64, error) {
	until := time.Now()
	since := until.Add(-25 * time.Hour)

	resp, err := d.Query(ctx, "derivatives", "futures_open_interest_sum", currency, "24h", &since, &until)

	if err != nil {
		return 0, err
	}

	return resp.Last().Value, nil
}

// query last market cap in usd
// https://docs.glassnode.com/api/market#market-cap
func (d *DataSource) QueryMarketCapInUSD(ctx context.Context, currency string) (float64, error) {
	until := time.Now()
	since := until.Add(-25 * time.Hour)

	resp, err := d.Query(ctx, "market", "marketcap_usd", currency, "24h", &since, &until)

	if err != nil {
		return 0, err
	}

	return resp.Last().Value, nil
}
