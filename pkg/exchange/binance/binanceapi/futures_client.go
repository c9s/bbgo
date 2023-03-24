package binanceapi

import (
	"net/url"

	"github.com/c9s/requestgen"
)

type FuturesRestClient struct {
	RestClient
}

const FuturesRestBaseURL = "https://fapi.binance.com"

func NewFuturesRestClient(baseURL string) *FuturesRestClient {
	if len(baseURL) == 0 {
		baseURL = FuturesRestBaseURL
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}

	return &FuturesRestClient{
		RestClient: RestClient{
			BaseAPIClient: requestgen.BaseAPIClient{
				BaseURL:    u,
				HttpClient: DefaultHttpClient,
			},
		},
	}
}
