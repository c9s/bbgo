package backpackapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

// PriceFilter describes the price constraints of a market.
// Only minPrice and tickSize are always present; the rest are market type dependent.
type PriceFilter struct {
	MinPrice fixedpoint.Value `json:"minPrice"`
	TickSize fixedpoint.Value `json:"tickSize"`

	MaxPrice fixedpoint.Value `json:"maxPrice,omitempty"`

	MaxMultiplier fixedpoint.Value `json:"maxMultiplier,omitempty"`
	MinMultiplier fixedpoint.Value `json:"minMultiplier,omitempty"`

	MaxImpactMultiplier fixedpoint.Value `json:"maxImpactMultiplier,omitempty"`
	MinImpactMultiplier fixedpoint.Value `json:"minImpactMultiplier,omitempty"`

	MeanMarkPriceBand *struct {
		MinMultiplier fixedpoint.Value `json:"minMultiplier"`
		MaxMultiplier fixedpoint.Value `json:"maxMultiplier"`
	} `json:"meanMarkPriceBand,omitempty"`

	MeanPremiumBand *struct {
		TolerancePct fixedpoint.Value `json:"tolerancePct"`
	} `json:"meanPremiumBand,omitempty"`

	BorrowEntryFeeMaxMultiplier fixedpoint.Value `json:"borrowEntryFeeMaxMultiplier,omitempty"`
	BorrowEntryFeeMinMultiplier fixedpoint.Value `json:"borrowEntryFeeMinMultiplier,omitempty"`

	MaxPriceUpdateMultiplier fixedpoint.Value `json:"maxPriceUpdateMultiplier,omitempty"`
	MinPriceUpdateMultiplier fixedpoint.Value `json:"minPriceUpdateMultiplier,omitempty"`
}

// QuantityFilter describes the quantity constraints of a market.
type QuantityFilter struct {
	MinQuantity fixedpoint.Value `json:"minQuantity"`
	StepSize    fixedpoint.Value `json:"stepSize"`

	MaxQuantity fixedpoint.Value `json:"maxQuantity,omitempty"`
}

type MarketFilters struct {
	Price    PriceFilter    `json:"price"`
	Quantity QuantityFilter `json:"quantity"`
}

// MarginFunction describes how the initial/maintenance margin fraction is computed for a
// futures market.
type MarginFunction struct {
	Type   string           `json:"type"`
	Base   fixedpoint.Value `json:"base"`
	Factor fixedpoint.Value `json:"factor"`
}

type Market struct {
	Symbol      string     `json:"symbol"`
	BaseSymbol  string     `json:"baseSymbol"`
	QuoteSymbol string     `json:"quoteSymbol"`
	MarketType  MarketType `json:"marketType"`

	Filters MarketFilters `json:"filters"`

	OrderBookState OrderBookState `json:"orderBookState"`
	CreatedAt      types.Time     `json:"createdAt"`
	Visible        bool           `json:"visible"`

	// futures only fields
	ImfFunction *MarginFunction `json:"imfFunction,omitempty"`
	MmfFunction *MarginFunction `json:"mmfFunction,omitempty"`

	// FundingInterval is in milliseconds
	FundingInterval uint64 `json:"fundingInterval,omitempty"`

	// FundingRateUpperBound and FundingRateLowerBound are expressed in basis points
	FundingRateUpperBound fixedpoint.Value `json:"fundingRateUpperBound,omitempty"`
	FundingRateLowerBound fixedpoint.Value `json:"fundingRateLowerBound,omitempty"`

	OpenInterestLimit   fixedpoint.Value `json:"openInterestLimit,omitempty"`
	PositionLimitWeight fixedpoint.Value `json:"positionLimitWeight,omitempty"`

	RwaMarketType string `json:"rwaMarketType,omitempty"`
}

//go:generate GetRequest -url "/api/v1/markets" -type GetMarketsRequest -responseType []Market
type GetMarketsRequest struct {
	client requestgen.APIClient

	// marketType filters the returned markets. The API accepts the parameter more than once;
	// this request sends at most one value, which covers the useful cases.
	// Omitting it returns the spot and perp markets.
	marketType *MarketType `param:"marketType"`
}

func (c *RestClient) NewGetMarketsRequest() *GetMarketsRequest {
	return &GetMarketsRequest{client: c}
}

//go:generate GetRequest -url "/api/v1/market" -type GetMarketRequest -responseType .Market
type GetMarketRequest struct {
	client requestgen.APIClient

	symbol string `param:"symbol,required"`
}

func (c *RestClient) NewGetMarketRequest() *GetMarketRequest {
	return &GetMarketRequest{client: c}
}
