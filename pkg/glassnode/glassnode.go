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
