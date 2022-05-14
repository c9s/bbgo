package v1

import (
	"github.com/c9s/requestgen"
)

//go:generate requestgen -method GET -url "/v1/cryptocurrency/listings/historical" -type ListingsHistoricalRequest -responseType Response -responseDataField Data -responseDataType []Data
type ListingsHistoricalRequest struct {
	Client requestgen.AuthenticatedAPIClient

	Date               string  `param:"date,query,required"`
	Start              *int    `param:"start,query" default:"1"`
	Limit              *int    `param:"limit,query" default:"100"`
	Convert            *string `param:"convert,query"`
	ConvertID          *string `param:"convert_id,query"`
	Sort               *string `param:"sort,query" default:"cmc_rank" validValues:"cmc_rank,name,symbol,market_cap,price,circulating_supply,total_supply,max_supply,num_market_pairs,volume_24h,percent_change_1h,percent_change_24h,percent_change_7d"`
	SortDir            *string `param:"sort_dir,query" validValues:"asc,desc"`
	CryptocurrencyType *string `param:"cryptocurrency_type,query" default:"all" validValues:"all,coins,tokens"`
	Aux                *string `param:"aux,query" default:"platform,tags,date_added,circulating_supply,total_supply,max_supply,cmc_rank,num_market_pairs"`
}

//go:generate requestgen -method GET -url "/v1/cryptocurrency/listings/latest" -type ListingsLatestRequest -responseType Response -responseDataField Data -responseDataType []Data
type ListingsLatestRequest struct {
	Client requestgen.AuthenticatedAPIClient

	Start                *int     `param:"start,query" default:"1"`
	Limit                *int     `param:"limit,query" default:"100"`
	PriceMin             *float64 `param:"price_min,query"`
	PriceMax             *float64 `param:"price_max,query"`
	MarketCapMin         *float64 `param:"market_cap_min,query"`
	MarketCapMax         *float64 `param:"market_cap_max,query"`
	Volume24HMin         *float64 `param:"volume_24h_min,query"`
	Volume24HMax         *float64 `param:"volume_24h_max,query"`
	CirculatingSupplyMin *float64 `param:"circulating_supply_min,query"`
	CirculatingSupplyMax *float64 `param:"circulating_supply_max,query"`
	PercentChange24HMin  *float64 `param:"percent_change_24h_min,query"`
	PercentChange24HMax  *float64 `param:"percent_change_24h_max,query"`
	Convert              *string  `param:"convert,query"`
	ConvertID            *string  `param:"convert_id,query"`
	Sort                 *string  `param:"sort,query" default:"market_cap" validValues:"name,symbol,date_added,market_cap,market_cap_strict,price,circulating_supply,total_supply,max_supply,num_market_pairs,volume_24h,percent_change_1h,percent_change_24h,percent_change_7d,market_cap_by_total_supply_strict,volume_7d,volume_30d"`
	SortDir              *string  `param:"sort_dir,query" validValues:"asc,desc"`
	CryptocurrencyType   *string  `param:"cryptocurrency_type,query" default:"all" validValues:"all,coins,tokens"`
	Tag                  *string  `param:"tag,query" default:"all" validValues:"all,defi,filesharing"`
	Aux                  *string  `param:"aux,query" default:"num_market_pairs,cmc_rank,date_added,tags,platform,max_supply,circulating_supply,total_supply"`
}

//go:generate requestgen -method GET -url "/v1/cryptocurrency/listings/new" -type ListingsNewRequest -responseType Response -responseDataField Data -responseDataType []Data
type ListingsNewRequest struct {
	Client requestgen.AuthenticatedAPIClient

	Start     *int    `param:"start,query" default:"1"`
	Limit     *int    `param:"limit,query" default:"100"`
	Convert   *string `param:"convert,query"`
	ConvertID *string `param:"convert_id,query"`
	SortDir   *string `param:"sort_dir,query" validValues:"asc,desc"`
}
