package v1

import (
	"encoding/json"
	"time"
)

type Response struct {
	Data   json.RawMessage `json:"data"`
	Status Status          `json:"status"`
}

type Data struct {
	ID                            int64            `json:"id"`
	Name                          string           `json:"name"`
	Symbol                        string           `json:"symbol"`
	Slug                          string           `json:"slug"`
	CmcRank                       int64            `json:"cmc_rank,omitempty"`
	IsActive                      bool             `json:"is_active,omitempty"`
	IsFiat                        int64            `json:"is_fiat,omitempty"`
	NumMarketPairs                int64            `json:"num_market_pairs"`
	CirculatingSupply             float64          `json:"circulating_supply"`
	TotalSupply                   float64          `json:"total_supply"`
	MaxSupply                     float64          `json:"max_supply"`
	LastUpdated                   time.Time        `json:"last_updated"`
	DateAdded                     time.Time        `json:"date_added"`
	Tags                          []string         `json:"tags"`
	SelfReportedCirculatingSupply float64          `json:"self_reported_circulating_supply,omitempty"`
	SelfReportedMarketCap         float64          `json:"self_reported_market_cap,omitempty"`
	Platform                      Platform         `json:"platform"`
	Quote                         map[string]Quote `json:"quote"`
}

type Quote struct {
	Price                 float64   `json:"price"`
	Volume24H             float64   `json:"volume_24h"`
	VolumeChange24H       float64   `json:"volume_change_24h"`
	PercentChange1H       float64   `json:"percent_change_1h"`
	PercentChange24H      float64   `json:"percent_change_24h"`
	PercentChange7D       float64   `json:"percent_change_7d"`
	MarketCap             float64   `json:"market_cap"`
	MarketCapDominance    float64   `json:"market_cap_dominance"`
	FullyDilutedMarketCap float64   `json:"fully_diluted_market_cap"`
	LastUpdated           time.Time `json:"last_updated"`
}

type Status struct {
	Timestamp    time.Time `json:"timestamp"`
	ErrorCode    int       `json:"error_code"`
	ErrorMessage string    `json:"error_message"`
	Elapsed      int       `json:"elapsed"`
	CreditCount  int       `json:"credit_count"`
}

type Platform struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Symbol       string `json:"symbol"`
	Slug         string `json:"slug"`
	TokenAddress string `json:"token_address"`
}
