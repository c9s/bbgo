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

// query last futures open interest
// https://docs.glassnode.com/api/derivatives#futures-open-interest
func (d *DataSource) QueryFuturesOpenInterest(ctx context.Context, currency string) (float64, error) {
	req := glassnodeapi.DerivativesRequest{
		Client: d.client,
		Asset:  currency,
		// 25 hours ago
		Since:    time.Now().Add(-25 * time.Hour).Unix(),
		Interval: glassnodeapi.Interval24h,
		Metric:   "futures_open_interest_sum",
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return 0, err
	}

	return resp.Last().Value, nil
}

// query last market cap in usd
// https://docs.glassnode.com/api/market#market-cap
func (d *DataSource) QueryMarketCapInUSD(ctx context.Context, currency string) (float64, error) {
	req := glassnodeapi.MarketRequest{
		Client: d.client,
		Asset:  currency,
		// 25 hours ago
		Since:    time.Now().Add(-25 * time.Hour).Unix(),
		Interval: glassnodeapi.Interval24h,
		Metric:   "marketcap_usd",
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return 0, err
	}

	return resp.Last().Value, nil
}
