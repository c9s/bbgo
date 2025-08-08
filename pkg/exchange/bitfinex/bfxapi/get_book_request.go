package bfxapi

import (
	"encoding/json"
	"fmt"

	"github.com/c9s/requestgen"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// GetBookRequest represents a request for Bitfinex order book.
//
//go:generate requestgen -type GetBookRequest -method GET -url "/v2/book/:symbol/:precision" -responseType .BookResponse
type GetBookRequest struct {
	client requestgen.APIClient

	symbol string `param:"symbol,slug"`

	precision string `param:"precision,slug" default:"P0"`

	length int `param:"length,number" default:"100"` // number of entries to return, default is 25
}

// NewGetBookRequest creates a new GetBookRequest.
func (c *Client) NewGetBookRequest() *GetBookRequest {
	return &GetBookRequest{client: c}
}

// BookEntry represents a trading pair book entry.
type BookEntry struct {
	Price  fixedpoint.Value
	Count  int64
	Amount fixedpoint.Value
}

// FundingBookEntry represents a funding currency book entry.
type FundingBookEntry struct {
	Rate   fixedpoint.Value
	Period int64
	Count  int64
	Amount fixedpoint.Value
}

// BookResponse holds the parsed book entries.
type BookResponse struct {
	BookEntries    []BookEntry
	FundingEntries []FundingBookEntry
}

// UnmarshalJSON parses the Bitfinex book response.
// It uses parseRawArray to fill BookEntries or FundingEntries.
func (r *BookResponse) UnmarshalJSON(data []byte) error {
	var rawEntries [][]json.RawMessage
	if err := json.Unmarshal(data, &rawEntries); err != nil {
		return err
	}
	for _, entry := range rawEntries {
		switch len(entry) {
		case 3:
			var be BookEntry
			if err := parseRawArray(entry, &be, 0); err != nil {
				logrus.Errorf("unable to parse book entry: %v, input: %s", err, entry)
				return err
			}
			r.BookEntries = append(r.BookEntries, be)
		case 4:
			var fe FundingBookEntry
			if err := parseRawArray(entry, &fe, 0); err != nil {
				logrus.Errorf("unable to parse funding book entry: %v, input: %s", err, entry)
				return fmt.Errorf("failed to parse funding book entry: %w", err)
			}
			r.FundingEntries = append(r.FundingEntries, fe)
		default:
			logrus.Errorf("unexpected entry length: %d, input: %s", len(entry), entry)
		}
	}
	return nil
}
