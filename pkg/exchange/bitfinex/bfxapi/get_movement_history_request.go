package bfxapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// GetMovementHistoryRequest represents a Bitfinex movements history request.
//
//go:generate requestgen -type GetMovementHistoryRequest -method POST -url "/v2/auth/r/movements/:currency/hist" -responseType []Movement
type GetMovementHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency string `param:"currency,slug"`

	start *time.Time `param:"start,millisecond"` // filter movements after this timestamp (ms)
	end   *time.Time `param:"end,millisecond"`   // filter movements before this timestamp (ms)

	limit *int    `param:"limit"` // max number of movements to return
	id    []int64 `param:"id"`    // filter by specific movement ID

	address *string `param:"address"` // filter by destination address
}

// NewGetMovementHistoryRequest creates a new GetMovementHistoryRequest.
func (c *Client) NewGetMovementHistoryRequest() *GetMovementHistoryRequest {
	return &GetMovementHistoryRequest{client: c}
}

// Movement represents a single deposit or withdrawal movement from Bitfinex API.
type Movement struct {
	ID                 int64
	Currency           string
	CurrencyName       string
	_                  any
	_                  any
	MtsStarted         types.MillisecondTimestamp
	MtsUpdated         types.MillisecondTimestamp
	_                  any
	_                  any
	Status             string
	_                  any
	_                  any
	Amount             fixedpoint.Value
	Fees               fixedpoint.Value
	_                  any
	_                  any
	DestinationAddress *string
	PaymentID          *string
	_                  any
	_                  any
	TransactionID      *string
	WithdrawNote       *string
}

// UnmarshalJSON parses the Bitfinex movement array response into Movement fields.
func (m *Movement) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, m, 0)
}
