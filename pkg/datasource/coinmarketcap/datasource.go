package coinmarketcap

import v1 "github.com/c9s/bbgo/pkg/datasource/coinmarketcap/v1"

type DataSource struct {
	client *v1.RestClient
}

func New(apiKey string) *DataSource {
	client := v1.New()
	client.Auth(apiKey)
	return &DataSource{client: client}
}
