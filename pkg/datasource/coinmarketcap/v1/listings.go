package v1

import "github.com/c9s/requestgen"

type RequestOptions struct {
	Start                int     `json:"start,omitempty"`
	Limit                int     `json:"limit,omitempty"`
	PriceMin             float64 `json:"price_min,omitempty"`
	PriceMax             float64 `json:"price_max,omitempty"`
	MarketCapMin         float64 `json:"market_cap_min,omitempty"`
	MarketCapMax         float64 `json:"market_cap_max,omitempty"`
	Volume24HMin         float64 `json:"volume_24h_min,omitempty"`
	Volume24HMax         float64 `json:"volume_24h_max,omitempty"`
	CirculatingSupplyMin float64 `json:"circulating_supply_min,omitempty"`
	CirculatingSupplyMax float64 `json:"circulating_supply_max,omitempty"`
	PercentChange24HMin  float64 `json:"percent_change_24h_min,omitempty"`
	PercentChange24HMax  float64 `json:"percent_change_24h_max,omitempty"`
	Convert              string  `json:"convert,omitempty"`
	ConvertID            string  `json:"convert_id,omitempty"`
	Sort                 string  `json:"sort,omitempty"`
	SortDir              string  `json:"sort_dir"`
	CryptocurrencyType   string  `json:"cryptocurrency_type,omitempty"`
	Tag                  string  `json:"tag,omitempty"`
	Aux                  string  `json:"aux,omitempty"`
}

//go:generate requestgen -type ListingsRequest -method GET -url "/v1/cryptocurrency/listings/latest" -responseType Response
type ListingsRequest struct {
	client requestgen.AuthenticatedAPIClient

	endpointType EndpointType `param:"endpointType,slug"`

	start int `param:"start,query"`
	limit int `param:"limit,query"`
	// priceMin             float64 `param:"price_min,query"`
	// priceMax             float64 `param:"price_max,query"`
	// marketCapMin         float64 `param:"market_cap_min,query"`
	// marketCapMax         float64 `param:"market_cap_max,query"`
	// volume24HMin         float64 `param:"volume_24h_min,query"`
	// volume24HMax         float64 `param:"volume_24h_max,query"`
	// circulatingSupplyMin float64 `param:"circulating_supply_min,query"`
	// circulatingSupplyMax float64 `param:"circulating_supply_max,query"`
	// percentChange24HMin  float64 `param:"percent_change_24h_min,query"`
	// percentChange24HMax  float64 `param:"percent_change_24h_max,query"`
	// convert            string `param:"convert,query"`
	// convertID          string `param:"convert_id,query"`
	sort               string `param:"sort,query"`
	sortDir            string `param:"sort_dir,query"`
	cryptocurrencyType string `param:"cryptocurrency_type,query"`
	tag                string `param:"tag,query"`
	aux                string `param:"aux,query"`
}

func NewListingsRequest(client requestgen.AuthenticatedAPIClient, options *RequestOptions) *ListingsRequest {
	if options.Start == 0 {
		options.Start = 1
	}

	if options.Limit == 0 {
		options.Limit = 100
	}

	if options.Sort == "" {
		options.Sort = "market_cap"
	}

	if options.SortDir == "" {
		options.SortDir = "asc"
	}

	if options.CryptocurrencyType == "" {
		options.CryptocurrencyType = "all"
	}

	if options.Tag == "" {
		options.Tag = "all"
	}

	if options.Aux == "" {
		options.Aux = "num_market_pairs,cmc_rank,date_added,tags,platform,max_supply,circulating_supply,total_supply"
	}

	return &ListingsRequest{
		client: client,

		start:              options.Start,
		limit:              options.Limit,
		sort:               options.Sort,
		sortDir:            options.SortDir,
		cryptocurrencyType: options.CryptocurrencyType,
		tag:                options.Tag,
		aux:                options.Aux,
	}
}
