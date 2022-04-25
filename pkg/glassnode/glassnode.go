package glassnode

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/glassnode/glassnodeapi"
)

type Glassnode struct {
	client *glassnodeapi.RestClient
}

func New(apiKey string) *Glassnode {
	client := glassnodeapi.NewRestClient()
	client.Auth(apiKey)

	return &Glassnode{client: client}
}

// query last futures open interest
// https://docs.glassnode.com/api/derivatives#futures-open-interest
func (g *Glassnode) QueryFuturesOpenInterest(ctx context.Context, currency string) (float64, error) {
	req := glassnodeapi.DerivativesRequest{
		Client: g.client,
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
func (g *Glassnode) QueryMarketCapInUSD(ctx context.Context, currency string) (float64, error) {
	req := glassnodeapi.MarketRequest{
		Client: g.client,
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
