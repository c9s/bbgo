package bfxapi

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/c9s/requestgen"
)

type PublicTradeHistoryResponse struct {
	Trades        []PublicTrade
	FundingTrades []PublicFundingTrade
}

func (r *PublicTradeHistoryResponse) UnmarshalJSON(data []byte) error {
	var raws [][]json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return err
	}

	if len(raws) == 0 {
		return nil
	}

	if len(raws[0]) >= 5 {
		// If the first element has 6 items, it's likely a FundingTrade
		var fundingTrades []PublicFundingTrade
		for _, rawSlice := range raws {
			var ft PublicFundingTrade
			if err := parseRawArray(rawSlice, &ft, 0); err != nil {
				return err
			}

			fundingTrades = append(fundingTrades, ft)
		}

		r.FundingTrades = fundingTrades
		return nil
	} else if len(raws[0]) == 4 {
		// If the first element has 4 items, it's likely a PublicTrade
		var trades []PublicTrade
		for _, rawSlice := range raws {
			var mt PublicTrade
			if err := parseRawArray(rawSlice, &mt, 0); err != nil {
				return err
			}

			trades = append(trades, mt)
		}

		r.Trades = trades
		return nil
	}

	return fmt.Errorf("unexpected trade data format: %s", string(data))
}

// GetPublicTradeHistoryBySymbolRequest
// API: https://docs.bitfinex.com/reference/rest-public-trades
//
//go:generate requestgen -type GetPublicTradeHistoryBySymbolRequest -method POST -url "/v2/trades/:symbol/hist" -responseType .PublicTradeHistoryResponse
type GetPublicTradeHistoryBySymbolRequest struct {
	client requestgen.APIClient

	symbol string `param:"symbol,slug"` // Trading pair symbol (e.g., "tBTCUSD")

	start *time.Time `param:"start,milliseconds"`   // Start timestamp in ms
	end   *time.Time `param:"end,milliseconds"`     // End timestamp in ms
	limit *int       `param:"limit" default:"2500"` // Limit number of results

	sort *int `param:"sort" default:"-1"` // Sort order: -1 for descending, 1 for ascending
}

func (c *Client) NewGetPublicTradeHistoryBySymbolRequest() *GetPublicTradeHistoryBySymbolRequest {
	return &GetPublicTradeHistoryBySymbolRequest{client: c}
}
