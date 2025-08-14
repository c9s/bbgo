package bfxapi

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/boolint"
)

type FundingService struct {
	*Client
}

type FundingInfoDetails struct {
	YieldLoan    fixedpoint.Value
	YieldLend    fixedpoint.Value
	DurationLoan float64
	DurationLend float64
}

// UnmarshalJSON parses the JSON array into the FundingInfoDetails struct fields.
func (d *FundingInfoDetails) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, d, 0)
}

// FundingOffer represents the funding offer details in the response.
type FundingOffer struct {
	ID          int64                      // offer ID
	Symbol      string                     // funding currency symbol
	CreatedAt   types.MillisecondTimestamp // creation timestamp
	UpdatedAt   types.MillisecondTimestamp // update timestamp
	Amount      fixedpoint.Value           // amount
	AmountOrig  fixedpoint.Value           // original amount
	OfferType   FundingOfferType           // offer type
	_           any
	_           any
	Flags       *int64             // flags
	OfferStatus FundingOfferStatus // offer status
	_           any
	_           any
	_           any
	Rate        fixedpoint.Value // rate
	Period      fixedpoint.Value // period in days
	Notify      *boolint.Value   // notify
	Hidden      *boolint.Value   // hidden
	_           any
	Renew       boolint.Value // auto renew
	Extra       any
}

// UnmarshalJSON parses the funding offer array into FundingOffer struct.
func (f *FundingOffer) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, f, 0)
}

// FundingOfferType represents the type of funding order for Bitfinex API.
type FundingOfferType string

const (
	FundingOfferTypeLimit       FundingOfferType = "LIMIT"       // Place an order at an explicit, static rate
	FundingOfferTypeFRRDeltaFix FundingOfferType = "FRRDELTAFIX" // Place an order at an implicit, static rate, relative to the FRR
	FundingOfferTypeFRRDeltaVar FundingOfferType = "FRRDELTAVAR" // Place an order at an implicit, dynamic rate, relative to the FRR
)

// String returns the string representation of the FundingOfferType.
func (t FundingOfferType) String() string {
	return string(t)
}

// FundingOfferStatus represents the status of a funding offer.
type FundingOfferStatus string

const (
	FundingOfferStatusActive          FundingOfferStatus = "ACTIVE"
	FundingOfferStatusPartiallyFilled FundingOfferStatus = "PARTIALLY FILLED"
)

// String returns the string representation of the FundingOfferStatus.
func (s FundingOfferStatus) String() string {
	return string(s)
}

// FundingTrade represents a single funding trade returned by Bitfinex.
type FundingTrade struct {
	ID        int64
	Currency  string
	CreatedAt types.MillisecondTimestamp
	OfferID   int64
	Amount    fixedpoint.Value
	Rate      fixedpoint.Value
	Period    int
	// Placeholder field for the last element, nullable
	Placeholder any
}

// UnmarshalJSON parses the JSON array into the FundingTrade struct fields.
func (t *FundingTrade) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, t, 0)
}
