package coinmarketcap

import (
	"context"

	v1 "github.com/c9s/bbgo/pkg/datasource/coinmarketcap/v1"
)

type DataSource struct {
	client *v1.RestClient
}

func New(apiKey string) *DataSource {
	client := v1.New()
	client.Auth(apiKey)
	return &DataSource{client: client}
}

func (d *DataSource) QueryMarketCapInUSD(ctx context.Context, limit int) (map[string]float64, error) {
	req := v1.ListingsLatestRequest{
		Client: d.client,
		Limit:  &limit,
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	marketcaps := make(map[string]float64)
	for _, data := range resp {
		marketcaps[data.Symbol] = data.Quote["USD"].MarketCap
	}

	return marketcaps, nil
}
