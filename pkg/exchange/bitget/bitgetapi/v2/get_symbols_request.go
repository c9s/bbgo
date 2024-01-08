package bitgetapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type SymbolStatus string

const (
	// SymbolStatusOffline represent market is suspended, users cannot trade.
	SymbolStatusOffline SymbolStatus = "offline"

	// SymbolStatusGray represents market is online, but user trading is not available.
	SymbolStatusGray SymbolStatus = "gray"

	// SymbolStatusOnline trading begins, users can trade.
	SymbolStatusOnline SymbolStatus = "online"
)

type Symbol struct {
	Symbol              string           `json:"symbol"`
	BaseCoin            string           `json:"baseCoin"`
	QuoteCoin           string           `json:"quoteCoin"`
	MinTradeAmount      fixedpoint.Value `json:"minTradeAmount"`
	MaxTradeAmount      fixedpoint.Value `json:"maxTradeAmount"`
	TakerFeeRate        fixedpoint.Value `json:"takerFeeRate"`
	MakerFeeRate        fixedpoint.Value `json:"makerFeeRate"`
	PricePrecision      fixedpoint.Value `json:"pricePrecision"`
	QuantityPrecision   fixedpoint.Value `json:"quantityPrecision"`
	QuotePrecision      fixedpoint.Value `json:"quotePrecision"`
	MinTradeUSDT        fixedpoint.Value `json:"minTradeUSDT"`
	Status              SymbolStatus     `json:"status"`
	BuyLimitPriceRatio  fixedpoint.Value `json:"buyLimitPriceRatio"`
	SellLimitPriceRatio fixedpoint.Value `json:"sellLimitPriceRatio"`
}

//go:generate GetRequest -url "/api/v2/spot/public/symbols" -type GetSymbolsRequest -responseDataType []Symbol
type GetSymbolsRequest struct {
	client requestgen.APIClient
}

func (c *Client) NewGetSymbolsRequest() *GetSymbolsRequest {
	return &GetSymbolsRequest{client: c.Client}
}
